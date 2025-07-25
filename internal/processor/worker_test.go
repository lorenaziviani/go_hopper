package processor

import (
	"sync"
	"testing"
)

// TestRaceCondition demonstrates race condition in unsafe metrics
func TestRaceCondition(t *testing.T) {
	metrics := NewMetrics()

	// Simulate concurrent access to unsafe metrics
	var wg sync.WaitGroup
	numGoroutines := 100
	iterations := 1000

	// This will likely trigger race condition
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.UnsafeIncrement("test_counter")
			}
		}()
	}

	wg.Wait()

	// Expected value: numGoroutines * iterations
	// Actual value: likely less due to race condition
	expected := int64(numGoroutines * iterations)
	actual := metrics.unsafeMetrics["test_counter"]

	t.Logf("Expected: %d, Actual: %d", expected, actual)

	if actual != expected {
		t.Logf("Race condition detected! Expected %d but got %d", expected, actual)
	}
}

// TestSafeMetricsWithMutex tests thread-safe metrics with mutex
func TestSafeMetricsWithMutex(t *testing.T) {
	metrics := NewMetrics()

	var wg sync.WaitGroup
	numGoroutines := 100
	iterations := 1000

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.SafeIncrementWithMutex("test_counter")
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * iterations)
	actual := metrics.safeMetrics["test_counter"]

	if actual != expected {
		t.Errorf("Mutex failed! Expected %d but got %d", expected, actual)
	}

	t.Logf("Mutex test passed: Expected %d, Actual %d", expected, actual)
}

// TestSafeMetricsWithSyncMap tests thread-safe metrics with sync.Map
func TestSafeMetricsWithSyncMap(t *testing.T) {
	metrics := NewMetrics()

	var wg sync.WaitGroup
	numGoroutines := 100
	iterations := 1000

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.SafeIncrementWithSyncMap("test_counter")
			}
		}()
	}

	wg.Wait()

	// Get value from sync.Map
	var actual int64
	if value, loaded := metrics.syncMetrics.Load("test_counter"); loaded {
		actual = value.(int64)
	}

	expected := int64(numGoroutines * iterations)

	if actual != expected {
		t.Errorf("Sync.Map failed! Expected %d but got %d", expected, actual)
	}

	t.Logf("Sync.Map test passed: Expected %d, Actual %d", expected, actual)
}

// TestAtomicMetrics tests thread-safe metrics with atomic operations
func TestAtomicMetrics(t *testing.T) {
	metrics := NewMetrics()

	var wg sync.WaitGroup
	numGoroutines := 100
	iterations := 1000

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.AtomicIncrement("processed")
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * iterations)
	actual := metrics.GetMetrics()["atomic_metrics"].(map[string]int64)["processed"]

	if actual != expected {
		t.Errorf("Atomic operations failed! Expected %d but got %d", expected, actual)
	}

	t.Logf("Atomic test passed: Expected %d, Actual %d", expected, actual)
}

// TestWorkerPoolMetrics tests metrics in worker pool context
func TestWorkerPoolMetrics(t *testing.T) {
	handler := NewDefaultMessageHandler()
	workerPool := NewWorkerPool(5, handler)

	// Simulate concurrent job submissions
	var wg sync.WaitGroup
	numJobs := 100

	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Simulate metrics update
			workerPool.metrics.AtomicIncrement("processed")
		}()
	}

	wg.Wait()

	stats := workerPool.GetStats()
	metrics := stats["metrics"].(map[string]interface{})
	atomicMetrics := metrics["atomic_metrics"].(map[string]int64)

	expected := int64(numJobs)
	actual := atomicMetrics["processed"]

	if actual != expected {
		t.Errorf("Worker pool metrics failed! Expected %d but got %d", expected, actual)
	}

	t.Logf("Worker pool metrics test passed: Expected %d, Actual %d", expected, actual)
}

// TestConcurrentMetricsAccess tests all metrics methods concurrently
func TestConcurrentMetricsAccess(t *testing.T) {
	metrics := NewMetrics()

	var wg sync.WaitGroup
	numGoroutines := 50
	iterations := 100

	// Test all metrics methods concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Unsafe (race condition prone)
				metrics.UnsafeIncrement("unsafe_counter")

				// Safe with mutex
				metrics.SafeIncrementWithMutex("mutex_counter")

				// Safe with sync.Map
				metrics.SafeIncrementWithSyncMap("syncmap_counter")

				// Safe with atomic
				metrics.AtomicIncrement("processed")
			}
		}()
	}

	wg.Wait()

	// Get all metrics
	allMetrics := metrics.GetMetrics()

	expected := int64(numGoroutines * iterations)

	// Check unsafe metrics (likely incorrect due to race condition)
	unsafeMetrics := allMetrics["unsafe_metrics"].(map[string]int64)
	unsafeActual := unsafeMetrics["unsafe_counter"]
	t.Logf("Unsafe metrics: Expected %d, Actual %d (race condition expected)", expected, unsafeActual)

	// Check safe metrics with mutex
	safeMetrics := allMetrics["safe_metrics"].(map[string]int64)
	safeActual := safeMetrics["mutex_counter"]
	if safeActual != expected {
		t.Errorf("Mutex metrics failed! Expected %d but got %d", expected, safeActual)
	}

	// Check sync.Map metrics
	syncMetrics := allMetrics["sync_metrics"].(map[string]int64)
	syncActual := syncMetrics["syncmap_counter"]
	if syncActual != expected {
		t.Errorf("Sync.Map metrics failed! Expected %d but got %d", expected, syncActual)
	}

	// Check atomic metrics
	atomicMetrics := allMetrics["atomic_metrics"].(map[string]int64)
	atomicActual := atomicMetrics["processed"]
	if atomicActual != expected {
		t.Errorf("Atomic metrics failed! Expected %d but got %d", expected, atomicActual)
	}

	t.Logf("All safe metrics tests passed!")
}

// BenchmarkUnsafeMetrics benchmarks unsafe metrics (race condition prone)
func BenchmarkUnsafeMetrics(b *testing.B) {
	metrics := NewMetrics()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics.UnsafeIncrement("benchmark_counter")
		}
	})
}

// BenchmarkSafeMetricsWithMutex benchmarks safe metrics with mutex
func BenchmarkSafeMetricsWithMutex(b *testing.B) {
	metrics := NewMetrics()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics.SafeIncrementWithMutex("benchmark_counter")
		}
	})
}

// BenchmarkSafeMetricsWithSyncMap benchmarks safe metrics with sync.Map
func BenchmarkSafeMetricsWithSyncMap(b *testing.B) {
	metrics := NewMetrics()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics.SafeIncrementWithSyncMap("benchmark_counter")
		}
	})
}

// BenchmarkAtomicMetrics benchmarks atomic metrics
func BenchmarkAtomicMetrics(b *testing.B) {
	metrics := NewMetrics()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics.AtomicIncrement("processed")
		}
	})
}
