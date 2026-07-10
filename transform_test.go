package rill

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func universalMap[A, B any](ord bool, in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	if ord {
		return OrderedMap(in, n, f)
	}
	return Map(in, n, f)
}

func TestMap(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		th.TestLevels(t, []int{1, 5}, func(t *testing.T, n int) {

			t.Run("nil", func(t *testing.T) {
				out := universalMap(ord, nil, n, func(x int) (int, error) { return x, nil })
				th.ExpectValue(t, out, nil)
			})

			th.RunSynctest(t, "correctness", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 20), nil)
				in = replaceWithError(in, 15, fmt.Errorf("err015"))

				out := universalMap(ord, in, n, func(x int) (string, error) {
					th.SimulateWork(1*time.Second, 2*time.Second)

					if x == 5 || x == 6 {
						return "", fmt.Errorf("err%03d", x)
					}
					return fmt.Sprintf("%03d", x), nil
				})

				outSlice := toItemSlice(out)

				var expectedSlice []Item[string]
				for i := range 20 {
					if i == 5 || i == 6 || i == 15 {
						expectedSlice = appendErr(expectedSlice, fmt.Errorf("err%03d", i))
						continue
					}
					expectedSlice = appendVal(expectedSlice, fmt.Sprintf("%03d", i))
				}

				th.ExpectElementsMatch(t, outSlice, expectedSlice)
			})

			th.RunSynctest(t, "ordering", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 100), nil)

				out := universalMap(ord, in, n, func(x int) (int, error) {
					th.SimulateWork(1*time.Second, 2*time.Second)
					if x%7 == 0 {
						time.Sleep(10 * time.Second) // force out-of-order completion
					}

					if x%2 == 1 {
						return 0, fmt.Errorf("%03d-err", x)
					}
					return x, nil
				})

				outSlice := toUnifiedStringSlice(out, "%03d")

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}
			})

		})
	})
}

func universalFilter(ord bool, in <-chan Try[int], n int, f func(int) (bool, error)) <-chan Try[int] {
	if ord {
		return OrderedFilter(in, n, f)
	}
	return Filter(in, n, f)
}

func TestFilter(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		th.TestLevels(t, []int{1, 5}, func(t *testing.T, n int) {

			t.Run("nil", func(t *testing.T) {
				out := universalFilter(ord, nil, n, func(x int) (bool, error) { return true, nil })
				th.ExpectValue(t, out, nil)
			})

			th.RunSynctest(t, "correctness", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 20), nil)
				in = replaceWithError(in, 15, fmt.Errorf("err015"))

				out := universalFilter(ord, in, n, func(x int) (bool, error) {
					th.SimulateWork(1*time.Second, 2*time.Second)

					if x == 5 || x == 6 {
						return x == 5, fmt.Errorf("err%03d", x)
					}
					return x%2 == 0, nil
				})

				outSlice := toItemSlice(out)

				var expectedSlice []Item[int]
				for i := range 20 {
					if i == 5 || i == 6 || i == 15 {
						expectedSlice = appendErr(expectedSlice, fmt.Errorf("err%03d", i))
						continue
					}
					if i%2 == 0 {
						expectedSlice = appendVal(expectedSlice, i)
					}
				}

				th.ExpectElementsMatch(t, outSlice, expectedSlice)
			})

			th.RunSynctest(t, "ordering", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 100), nil)

				out := universalFilter(ord, in, n, func(x int) (bool, error) {
					th.SimulateWork(1*time.Second, 2*time.Second)
					if x%7 == 0 {
						time.Sleep(10 * time.Second) // force out-of-order completion
					}

					switch x % 3 {
					case 2:
						return false, fmt.Errorf("%03d-err", x)
					case 1:
						return false, nil
					default:
						return true, nil
					}
				})

				outSlice := toUnifiedStringSlice(out, "%03d")

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}
			})

		})
	})
}

func universalFilterMap[A, B any](ord bool, in <-chan Try[A], n int, f func(A) (B, bool, error)) <-chan Try[B] {
	if ord {
		return OrderedFilterMap(in, n, f)
	}
	return FilterMap(in, n, f)
}

func TestFilterMap(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		th.TestLevels(t, []int{1, 5}, func(t *testing.T, n int) {

			t.Run("nil", func(t *testing.T) {
				out := universalFilterMap(ord, nil, n, func(x int) (int, bool, error) { return x, true, nil })
				th.ExpectValue(t, out, nil)
			})

			th.RunSynctest(t, "correctness", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 20), nil)
				in = replaceWithError(in, 15, fmt.Errorf("err015"))

				out := universalFilterMap(ord, in, n, func(x int) (string, bool, error) {
					th.SimulateWork(1*time.Second, 2*time.Second)

					if x == 5 || x == 6 {
						return "dummy", x == 5, fmt.Errorf("err%03d", x)
					}
					return fmt.Sprintf("%03d", x), x%2 == 0, nil
				})

				outSlice := toItemSlice(out)

				var expectedSlice []Item[string]
				for i := range 20 {
					if i == 5 || i == 6 || i == 15 {
						expectedSlice = appendErr(expectedSlice, fmt.Errorf("err%03d", i))
						continue
					}
					if i%2 == 0 {
						expectedSlice = appendVal(expectedSlice, fmt.Sprintf("%03d", i))
					}
				}

				th.ExpectElementsMatch(t, outSlice, expectedSlice)
			})

			th.RunSynctest(t, "ordering", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 100), nil)

				out := universalFilterMap(ord, in, n, func(x int) (int, bool, error) {
					th.SimulateWork(1*time.Second, 2*time.Second)
					if x%7 == 0 {
						time.Sleep(10 * time.Second) // force out-of-order completion
					}

					switch x % 3 {
					case 2:
						return x, false, fmt.Errorf("%03d-err", x)
					case 1:
						return x, false, nil
					default:
						return x, true, nil
					}
				})

				outSlice := toUnifiedStringSlice(out, "%03d")

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}
			})

		})
	})
}

func universalFlatMap[A, B any](ord bool, in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	if ord {
		return OrderedFlatMap(in, n, f)
	}
	return FlatMap(in, n, f)
}

func TestFlatMap(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		th.TestLevels(t, []int{1, 5}, func(t *testing.T, n int) {

			t.Run("nil", func(t *testing.T) {
				out := universalFlatMap(ord, nil, n, func(x int) <-chan Try[string] { return nil })
				th.ExpectValue(t, out, nil)
			})

			th.RunSynctest(t, "correctness", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 20), nil)
				in = replaceWithError(in, 5, fmt.Errorf("err005I"))
				in = replaceWithError(in, 15, fmt.Errorf("err015I"))

				// Each item emits a mini-stream of interleaved values and errors
				out := universalFlatMap(ord, in, n, func(x int) <-chan Try[string] {
					th.SimulateWork(1*time.Second, 2*time.Second)

					return Generate(func(send func(string), sendErr func(error)) {
						send(fmt.Sprintf("%03dA", x))
						sendErr(fmt.Errorf("err%03dA", x))

						th.SimulateWork(1*time.Second, 2*time.Second)

						send(fmt.Sprintf("%03dB", x))
						sendErr(fmt.Errorf("err%03dB", x))
					})
				})

				outSlice := toItemSlice(out)

				var expectedSlice []Item[string]
				for i := range 20 {
					if i == 5 || i == 15 {
						expectedSlice = appendErr(expectedSlice, fmt.Errorf("err%03dI", i))
						continue
					}

					expectedSlice = appendVal(expectedSlice, fmt.Sprintf("%03dA", i), fmt.Sprintf("%03dB", i))
					expectedSlice = appendErr(expectedSlice, fmt.Errorf("err%03dA", i), fmt.Errorf("err%03dB", i))
				}

				th.ExpectElementsMatch(t, outSlice, expectedSlice)
			})

			th.RunSynctest(t, "ordering", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 100), nil)

				out := universalFlatMap(ord, in, n, func(x int) <-chan Try[string] {
					th.SimulateWork(1*time.Second, 2*time.Second)
					if x%7 == 0 {
						time.Sleep(10 * time.Second) // force out-of-order completion
					}

					return Generate(func(send func(string), sendErr func(error)) {
						send(fmt.Sprintf("%03d-A", x))
						sendErr(fmt.Errorf("%03d-A-err", x))

						if x%9 == 0 {
							// A gap between this item's A and B outputs, so the assertions below also
							// cover the case where a single item yields its values non-contiguously.
							th.SimulateWork(2*time.Second, 4*time.Second)
						}

						send(fmt.Sprintf("%03d-B", x))
						sendErr(fmt.Errorf("%03d-B-err", x))
					})
				})

				outSlice := toUnifiedStringSlice(out, "%s")

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}
			})

		})
	})
}

func universalCatch(ord bool, in <-chan Try[int], n int, f func(error) error) <-chan Try[int] {
	if ord {
		return OrderedCatch(in, n, f)
	}
	return Catch(in, n, f)
}

func TestCatch(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		th.TestLevels(t, []int{1, 5}, func(t *testing.T, n int) {

			t.Run("nil", func(t *testing.T) {
				out := universalCatch(ord, nil, n, func(err error) error { return nil })
				th.ExpectValue(t, out, nil)
			})

			th.RunSynctest(t, "correctness", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 20), nil)
				in = replaceWithError(in, 5, fmt.Errorf("err05"))
				in = replaceWithError(in, 10, fmt.Errorf("err10"))
				in = replaceWithError(in, 15, fmt.Errorf("err15"))

				out := universalCatch(ord, in, n, func(err error) error {
					th.SimulateWork(1*time.Second, 2*time.Second)

					if err.Error() == "err05" {
						return nil // handled
					}
					if err.Error() == "err10" {
						return fmt.Errorf("%w wrapped", err) // wrapped/replaced
					}

					return err // leave as is
				})

				outSlice := toItemSlice(out)

				var expectedSlice []Item[int]
				for i := range 20 {
					switch i {
					case 5:
						// skip
					case 10:
						expectedSlice = appendErr(expectedSlice, fmt.Errorf("err10 wrapped")) // we only need messages to be equal
					case 15:
						expectedSlice = appendErr(expectedSlice, fmt.Errorf("err15"))
					default:
						expectedSlice = appendVal(expectedSlice, i)
					}
				}

				th.ExpectElementsMatch(t, outSlice, expectedSlice)
			})

			th.RunSynctest(t, "ordering", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 100), nil)

				in = OrderedMap(in, 1, func(x int) (int, error) {
					if x%2 == 0 {
						return x, fmt.Errorf("%03d-err", x)
					}
					return x, nil
				})

				var i atomic.Int64
				out := universalCatch(ord, in, n, func(err error) error {
					th.SimulateWork(1*time.Second, 2*time.Second)
					if i.Add(1)%7 == 0 {
						time.Sleep(10 * time.Second) // force out-of-order completion
					}
					return fmt.Errorf("%w wrapped", err)
				})

				outSlice := toUnifiedStringSlice(out, "%03d")

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}

			})

		})
	})
}
