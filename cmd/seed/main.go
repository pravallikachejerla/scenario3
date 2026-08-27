// Command seed generates deterministic synthetic order-management
// demonstration/fixture data with a fixed seed and recorded SHA-256
// hash. Small/medium profiles are used for demonstration and
// functional regression; the large profile is used for concurrency
// and capacity evaluation (see internal/dependencies, internal/workers).
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
)

const DefaultSeed = 20260812

type Profile string

const (
	Small  Profile = "small"
	Medium Profile = "medium"
	Large  Profile = "large"
)

type SeedOrder struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	CustomerID string `json:"customer_id"`
	SKU        string `json:"sku"`
	Quantity   int    `json:"quantity"`
}

type Dataset struct {
	Seed      int64       `json:"seed"`
	Profile   Profile     `json:"profile"`
	Tenants   []string    `json:"tenants"`
	Orders    []SeedOrder `json:"orders"`
	SHA256    string      `json:"sha256"`
}

func build(seed int64, profile Profile) Dataset {
	r := rand.New(rand.NewSource(seed))
	n := map[Profile]int{Small: 200, Medium: 5000, Large: 20000}[profile]
	tenants := []string{"tenant-a", "tenant-b", "tenant-c"}
	ds := Dataset{Seed: seed, Profile: profile, Tenants: tenants}
	for i := 0; i < n; i++ {
		ds.Orders = append(ds.Orders, SeedOrder{
			ID: fmt.Sprintf("ord-%s-%06d", profile, i), TenantID: tenants[r.Intn(len(tenants))],
			CustomerID: fmt.Sprintf("cust-%04d", r.Intn(500)), SKU: fmt.Sprintf("SKU-%03d", r.Intn(50)),
			Quantity: 1 + r.Intn(5),
		})
	}
	return ds
}

func main() {
	out := flag.String("out", "testdata/fixtures", "output directory")
	profile := flag.String("profile", "medium", "small|medium|large")
	flag.Parse()

	ds := build(DefaultSeed, Profile(*profile))
	b, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(b)
	ds.SHA256 = hex.EncodeToString(sum[:])
	b, _ = json.MarshalIndent(ds, "", "  ")

	if err := os.MkdirAll(*out, 0o755); err != nil {
		panic(err)
	}
	path := filepath.Join(*out, string(*profile)+".json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s (%d orders, sha256=%s)\n", path, len(ds.Orders), ds.SHA256)
}
