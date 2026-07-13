package main

import (
	"context"
	"fmt"
	"log"

	"github.com/glancedb/glancedb/api"
	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/table"
)

func main() {
	ctx := context.Background()

	// 1. Connect to (or create) a database directory.
	db, err := api.Connect("./golancedb_demo")
	if err != nil {
		log.Fatalf("Connect: %v", err)
	}
	defer db.Close()

	// 2. Create a table schema with an id, a 4-dimensional embedding vector, and a category.
	schema := table.NewSchema([]*table.Field{
		{Name: "id", Type: encode.TypeInt64, Nullable: false},
		{Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: 4},
		{Name: "category", Type: encode.TypeString, Nullable: true},
	})

	tbl, err := db.CreateTable(ctx, "documents", schema)
	if err != nil {
		log.Fatalf("CreateTable: %v", err)
	}

	// 3. Insert 10 document embeddings.
	//    Each embedding vector is [docID, 0, 0, 0] so documents form a line along the first axis.
	for i := 0; i < 10; i++ {
		batch := table.NewRecordBatch(schema, 1)
		batch.SetColumn(0, []int64{int64(i)})
		batch.SetColumn(1, []float32{float32(i), 0, 0, 0})
		category := "science"
		if i%2 == 0 {
			category = "art"
		}
		batch.SetColumn(2, []string{category})

		if err := tbl.Insert(ctx, batch); err != nil {
			log.Fatalf("Insert row %d: %v", i, err)
		}
	}
	fmt.Println("Inserted 10 documents.")

	// 4. Vector search: find the top-3 documents closest to [3, 0, 0, 0].
	fmt.Println("\n--- Vector Search (top-3 nearest to [3,0,0,0]) ---")
	q := api.NewQuery(api.Vector([]float32{3, 0, 0, 0}).Column("embedding")).TopK(3).Build()
	results, err := tbl.Search(ctx, q)
	if err != nil {
		log.Fatalf("Search: %v", err)
	}
	for _, r := range results {
		fmt.Printf("  RowID=%d  Score=%.4f\n", r.RowID, r.Score)
	}

	// 5. Hybrid search: only "science" documents closest to [3, 0, 0, 0].
	fmt.Println("\n--- Hybrid Search (science only, top-2 nearest to [3,0,0,0]) ---")
	q2 := api.NewQuery(api.Vector([]float32{3, 0, 0, 0}).Column("embedding")).
		Filter(api.EQ("category", "science")).
		TopK(2).
		Build()
	results2, err := tbl.Search(ctx, q2)
	if err != nil {
		log.Fatalf("HybridSearch: %v", err)
	}
	for _, r := range results2 {
		fmt.Printf("  RowID=%d  Score=%.4f (category=science)\n", r.RowID, r.Score)
	}

	// 6. Schema evolution: add a metadata column.
	field := &table.Field{Name: "tags", Type: encode.TypeString, Nullable: true}
	if err := tbl.AddColumn(ctx, field); err != nil {
		log.Fatalf("AddColumn: %v", err)
	}
	fmt.Printf("\nAdded 'tags' column. Schema now has %d fields.\n", tbl.Schema().NumFields())

	// 7. Check total document count.
	n, _ := tbl.NumRows(ctx)
	fmt.Printf("Total documents: %d\n", n)

	fmt.Println("\nDone!")
}
