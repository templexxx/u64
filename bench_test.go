package u64

import (
	"sync/atomic"
	"testing"
)

type bench struct {
	setup func(*testing.B, *Set)
	perG  func(b *testing.B, pb *testing.PB, i uint64, s *Set)
}

const initCap = 1 << 10 // Avoiding no slot to add.

func benchSet(b *testing.B, bench bench) {
	s, _ := New(initCap)
	b.Run("", func(b *testing.B) {
		if bench.setup != nil {
			bench.setup(b, s)
		}

		b.ResetTimer()

		var i uint64
		b.RunParallel(func(pb *testing.PB) {
			id := atomic.AddUint64(&i, 1) - 1
			bench.perG(b, pb, id*uint64(b.N), s)
		})
	})
}

func BenchmarkContainsMostlyHits(b *testing.B) {
	const hits, misses = 1023, 1 // Using const for helping compiler to optimize module.

	benchSet(b, bench{
		setup: func(_ *testing.B, s *Set) {
			for i := uint64(0); i < hits; i++ {
				_ = s.Add(i)
			}
			// Prime the set to get it into a steady state.
			for i := uint64(0); i < hits*2; i++ {
				s.Contains(i % hits)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i uint64, s *Set) {
			for ; pb.Next(); i++ {
				s.Contains(i % (hits + misses))
			}
		},
	})
}

func BenchmarkContainsMostlyMisses(b *testing.B) {
	const hits, misses = 1, 1023

	benchSet(b, bench{
		setup: func(_ *testing.B, s *Set) {
			for i := uint64(0); i < hits; i++ {
				_ = s.Add(i)
			}
			// Prime the set to get it into a steady state.
			for i := uint64(0); i < hits*2; i++ {
				s.Contains(i % hits)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i uint64, s *Set) {
			for ; pb.Next(); i++ {
				s.Contains(i % (hits + misses))
			}
		},
	})
}

func BenchmarkAddContainsBalanced(b *testing.B) {
	const hits, misses = 128, 128

	benchSet(b, bench{
		setup: func(b *testing.B, s *Set) {

			for i := uint64(0); i < hits; i++ {
				_ = s.Add(i)
			}
			// Prime the set to get it into a steady state.
			for i := uint64(0); i < hits*2; i++ {
				s.Contains(i % hits)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i uint64, s *Set) {
			for ; pb.Next(); i++ {
				j := i % (hits + misses)
				if j < hits {
					if !s.Contains(j) {
						b.Fatalf("unexpected miss for %v", j)
					}
				} else {
					_ = s.Add(i)
				}
			}
		},
	})
}

func BenchmarkAddUnique(b *testing.B) {
	benchSet(b, bench{
		setup: func(b *testing.B, s *Set) {

		},

		perG: func(b *testing.B, pb *testing.PB, i uint64, s *Set) {
			for ; pb.Next(); i++ {
				_ = s.Add(i)
			}
		},
	})
}

func BenchmarkAddCollision(b *testing.B) {
	benchSet(b, bench{
		setup: func(_ *testing.B, s *Set) {
			_ = s.Add(1)
		},

		perG: func(b *testing.B, pb *testing.PB, i uint64, s *Set) {
			for ; pb.Next(); i++ {
				_ = s.Add(1)
			}
		},
	})
}

func BenchmarkRemoveCollision(b *testing.B) {
	benchSet(b, bench{
		setup: func(_ *testing.B, s *Set) {
			_ = s.Add(1)
		},

		perG: func(b *testing.B, pb *testing.PB, i uint64, s *Set) {
			for ; pb.Next(); i++ {
				s.Remove(1)
			}
		},
	})
}

func BenchmarkRange(b *testing.B) {
	const mapSize = 1 << 9

	benchSet(b, bench{
		setup: func(_ *testing.B, s *Set) {
			for i := uint64(0); i < mapSize; i++ {
				_ = s.Add(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i uint64, s *Set) {
			for ; pb.Next(); i++ {
				s.Range(func(key uint64) bool {
					return true
				})
			}
		},
	})
}
