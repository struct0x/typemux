package typemux_test

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/struct0x/typemux"
)

type Foo struct {
	ID   int
	Name string
}

var (
	regBench    = typemux.NewRegistry()
	sealedBench *typemux.SealedRegistry
)

var u64Sink uint64

func init() {
	typemux.RegisterFactory(regBench, "foo", func(data string) (Foo, error) {
		return Foo{Name: data}, nil
	})

	typemux.RegisterDispatch[Foo](regBench, func(ctx context.Context, foo Foo) error {
		atomic.AddUint64(&u64Sink, uint64(1))
		return nil
	})

	sealedBench = regBench.Seal()
}

func BenchmarkCreateType(b *testing.B) {
	b.SetParallelism(runtime.GOMAXPROCS(0))
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(b *testing.PB) {
		var localErr error
		for b.Next() {
			_, localErr = typemux.CreateType(regBench, "foo", "Bench")
		}
		_ = localErr
	})
}

func BenchmarkCreateTypeSealed(b *testing.B) {
	b.SetParallelism(runtime.GOMAXPROCS(0))
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(b *testing.PB) {
		var localErr error
		for b.Next() {
			_, localErr = typemux.CreateType(sealedBench, "foo", "Bench")
		}
		_ = localErr
	})
}

func BenchmarkDispatch(b *testing.B) {
	ctx := b.Context()

	b.SetParallelism(runtime.GOMAXPROCS(0))
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(b *testing.PB) {
		var localErr error
		for b.Next() {
			localErr = typemux.Dispatch(regBench, ctx, Foo{ID: 123, Name: "Bench"})
		}
		_ = localErr
	})
}

func BenchmarkDispatchSealed(b *testing.B) {
	ctx := b.Context()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(b *testing.PB) {
		var localErr error
		for b.Next() {
			localErr = typemux.Dispatch(sealedBench, ctx, Foo{ID: 123, Name: "Bench"})
		}
		_ = localErr
	})
}
