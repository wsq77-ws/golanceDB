package index

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/glancedb/glancedb/storage"
)

// IVFFlatIndex implements IVF + Flat indexing using K-Means clustering.
// Vectors are partitioned into clusters; search scans only the nProbes nearest clusters.
type IVFFlatIndex struct {
	centroids     [][]float32
	partitions    [][]int32
	vectors       []VectorRecord
	numPartitions int
	metric        DistanceMetric
	stats         IndexStats
	dim           int
	// NProbes controls how many nearest partitions are scanned at search time.
	// If zero, defaults to min(numPartitions, 10) at search time.
	NProbes int
}

var _ Index = (*IVFFlatIndex)(nil)

// NewIVFFlatIndex creates an IVFFlatIndex with the given number of partitions and metric.
func NewIVFFlatIndex(numPartitions int, metric DistanceMetric) *IVFFlatIndex {
	if numPartitions < 1 {
		numPartitions = 1
	}
	return &IVFFlatIndex{
		numPartitions: numPartitions,
		metric:        metric,
	}
}

// Type returns IndexTypeIVFFlat.
func (idx *IVFFlatIndex) Type() IndexType { return IndexTypeIVFFlat }

// Stats returns build statistics.
func (idx *IVFFlatIndex) Stats() IndexStats { return idx.stats }

// copyVector returns a copy of v.
func copyVector(v []float32) []float32 {
	out := make([]float32, len(v))
	copy(out, v)
	return out
}

// assignVectors assigns each vector to its nearest centroid and returns the centroid index.
func assignVectors(vectors []VectorRecord, centroids [][]float32, metric DistanceMetric) ([]int, error) {
	assignments := make([]int, len(vectors))
	for vi, v := range vectors {
		bestC := 0
		bestD := math.MaxFloat64
		for ci, c := range centroids {
			d, err := Distance(v.Vector, c, metric)
			if err != nil {
				return nil, fmt.Errorf("index: %w", err)
			}
			if d < bestD {
				bestD = d
				bestC = ci
			}
		}
		assignments[vi] = bestC
	}
	return assignments, nil
}

// Build constructs the IVF index from the given vectors using K-Means clustering.
func (idx *IVFFlatIndex) Build(ctx context.Context, vectors []VectorRecord) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	if len(vectors) == 0 {
		return fmt.Errorf("index: cannot build IVFFlat from empty vectors")
	}
	dim := len(vectors[0].Vector)
	if dim == 0 {
		return fmt.Errorf("index: vector dimension must be > 0")
	}
	for i, v := range vectors {
		if len(v.Vector) != dim {
			return fmt.Errorf("index: vector %d has dimension %d, want %d", i, len(v.Vector), dim)
		}
	}

	n := len(vectors)
	k := idx.numPartitions
	if k > n {
		k = n
	}

	// Initialize centroids: spread evenly across the data.
	centroids := make([][]float32, k)
	for i := 0; i < k; i++ {
		si := (i * n) / k
		centroids[i] = copyVector(vectors[si].Vector)
	}

	const maxIters = 25
	const convThreshold = 1e-6
	for iter := 0; iter < maxIters; iter++ {
		assignments, err := assignVectors(vectors, centroids, idx.metric)
		if err != nil {
			return fmt.Errorf("index: %w", err)
		}

		newCentroids := make([][]float32, k)
		counts := make([]int, k)
		for i := range newCentroids {
			newCentroids[i] = make([]float32, dim)
		}
		for vi, c := range assignments {
			for d := 0; d < dim; d++ {
				newCentroids[c][d] += vectors[vi].Vector[d]
			}
			counts[c]++
		}

		var moved float64
		for ci := 0; ci < k; ci++ {
			if counts[ci] == 0 {
				// Reinitialize empty centroid to a random data vector.
				newCentroids[ci] = copyVector(vectors[rand.Intn(n)].Vector)
				centroids[ci] = newCentroids[ci]
				continue
			}
			for d := 0; d < dim; d++ {
				newCentroids[ci][d] /= float32(counts[ci])
			}
			md, err := Distance(centroids[ci], newCentroids[ci], DistanceEuclidean)
			if err == nil {
				moved += md
			}
			centroids[ci] = newCentroids[ci]
		}

		if moved < convThreshold {
			break
		}
	}

	// Final assignment to build partitions matching the final centroids.
	assignments, err := assignVectors(vectors, centroids, idx.metric)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	partitions := make([][]int32, k)
	for vi, c := range assignments {
		partitions[c] = append(partitions[c], int32(vi))
	}

	idx.centroids = centroids
	idx.partitions = partitions
	idx.vectors = make([]VectorRecord, len(vectors))
	copy(idx.vectors, vectors)
	idx.numPartitions = k
	idx.dim = dim
	idx.stats = IndexStats{
		NumVectors:    int64(n),
		NumPartitions: k,
		IndexType:     IndexTypeIVFFlat,
	}
	return nil
}

// Search returns the top-k nearest neighbors by scanning the nProbes nearest partitions.
func (idx *IVFFlatIndex) Search(ctx context.Context, query []float32, k int, metric DistanceMetric) ([]SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("index: %w", err)
	}
	if len(idx.vectors) == 0 {
		return nil, fmt.Errorf("index: index not built or empty")
	}
	if len(query) != idx.dim {
		return nil, fmt.Errorf("index: query dimension %d does not match index dimension %d", len(query), idx.dim)
	}
	if k <= 0 {
		return []SearchResult{}, nil
	}

	nProbes := idx.NProbes
	if nProbes <= 0 {
		nProbes = idx.numPartitions
		if nProbes > 10 {
			nProbes = 10
		}
	}
	if nProbes > idx.numPartitions {
		nProbes = idx.numPartitions
	}

	// Rank centroids by distance to the query.
	type centDist struct {
		idx int
		d   float64
	}
	cdist := make([]centDist, idx.numPartitions)
	for i, c := range idx.centroids {
		d, err := Distance(query, c, metric)
		if err != nil {
			return nil, fmt.Errorf("index: %w", err)
		}
		cdist[i] = centDist{i, d}
	}
	sort.Slice(cdist, func(i, j int) bool {
		return cdist[i].d < cdist[j].d
	})

	// Scan the nProbes nearest partitions.
	var results []SearchResult
	for p := 0; p < nProbes; p++ {
		ci := cdist[p].idx
		for _, vi := range idx.partitions[ci] {
			d, err := Distance(query, idx.vectors[vi].Vector, metric)
			if err != nil {
				return nil, fmt.Errorf("index: %w", err)
			}
			results = append(results, SearchResult{RowID: idx.vectors[vi].RowID, Score: d})
		}
	}

	return TopK(results, k), nil
}

// ivfFlatSnapshot is the JSON-serializable form of an IVFFlatIndex.
type ivfFlatSnapshot struct {
	Centroids     [][]float32    `json:"centroids"`
	Partitions    [][]int32      `json:"partitions"`
	Vectors       []VectorRecord `json:"vectors"`
	NumPartitions int            `json:"num_partitions"`
	Metric        DistanceMetric `json:"metric"`
	Dim           int            `json:"dim"`
	NProbes       int            `json:"nprobes"`
}

// Save persists the index to the given path as JSON via the ObjectStore.
func (idx *IVFFlatIndex) Save(ctx context.Context, store storage.ObjectStore, path string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	snap := ivfFlatSnapshot{
		Centroids:     idx.centroids,
		Partitions:    idx.partitions,
		Vectors:       idx.vectors,
		NumPartitions: idx.numPartitions,
		Metric:        idx.metric,
		Dim:           idx.dim,
		NProbes:       idx.NProbes,
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	if err := store.Write(ctx, path, data); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	return nil
}

// Load reads the index from the given path via the ObjectStore.
func (idx *IVFFlatIndex) Load(ctx context.Context, store storage.ObjectStore, path string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	size, err := store.Size(ctx, path)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	data, err := store.Read(ctx, path, 0, size)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	var snap ivfFlatSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	idx.centroids = snap.Centroids
	idx.partitions = snap.Partitions
	idx.vectors = snap.Vectors
	idx.numPartitions = snap.NumPartitions
	idx.metric = snap.Metric
	idx.dim = snap.Dim
	idx.NProbes = snap.NProbes
	idx.stats = IndexStats{
		NumVectors:    int64(len(snap.Vectors)),
		NumPartitions: snap.NumPartitions,
		IndexType:     IndexTypeIVFFlat,
	}
	return nil
}
