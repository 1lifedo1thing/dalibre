package calibre

import (
	"crypto/rand"
	"database/sql/driver"
	"fmt"
	"sort"
	"strings"
	"sync"

	"modernc.org/sqlite"
)

// calibre registers these functions on its connection; its triggers and
// views call them, so a write to books or series fails without them.
var registerOnce sync.Once

func registerFunctions() {
	registerOnce.Do(func() {
		sqlite.MustRegisterDeterministicScalarFunction("title_sort", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return TitleSort(asString(args[0])), nil
		})
		sqlite.MustRegisterScalarFunction("uuid4", 0, func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
			return newUUID(), nil
		})
		// books_list_filter is calibre's in-memory search restriction; outside
		// calibre every book passes.
		sqlite.MustRegisterScalarFunction("books_list_filter", 1, func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
			return int64(1), nil
		})
		agg := func(name string, nargs int32, mk func() *concatAgg) {
			sqlite.MustRegisterFunction(name, &sqlite.FunctionImpl{
				NArgs:         nargs,
				MakeAggregate: func(sqlite.FunctionContext) (sqlite.AggregateFunction, error) { return mk(), nil },
			})
		}
		agg("concat", 1, func() *concatAgg { return &concatAgg{sep: ","} })
		agg("sortconcat", 2, func() *concatAgg { return &concatAgg{sep: ",", sorted: true} })
		agg("sortconcat_bar", 2, func() *concatAgg { return &concatAgg{sep: "|", sorted: true} })
		agg("sortconcat_amper", 2, func() *concatAgg { return &concatAgg{sep: "&", sorted: true} })
		agg("identifiers_concat", 2, func() *concatAgg { return &concatAgg{sep: ",", pair: true} })
	})
}

func asString(v driver.Value) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type concatItem struct {
	key int64
	val string
}

// concatAgg implements calibre's concat, sortconcat* and identifiers_concat.
type concatAgg struct {
	sep    string
	sorted bool // first argument is an ordering key
	pair   bool // "type:val"
	items  []concatItem
}

func (a *concatAgg) Step(_ *sqlite.FunctionContext, args []driver.Value) error {
	switch {
	case a.pair:
		a.items = append(a.items, concatItem{val: asString(args[0]) + ":" + asString(args[1])})
	case a.sorted:
		k, _ := args[0].(int64)
		a.items = append(a.items, concatItem{key: k, val: asString(args[1])})
	default:
		a.items = append(a.items, concatItem{val: asString(args[0])})
	}
	return nil
}

func (a *concatAgg) WindowInverse(*sqlite.FunctionContext, []driver.Value) error { return nil }

func (a *concatAgg) WindowValue(*sqlite.FunctionContext) (driver.Value, error) {
	if len(a.items) == 0 {
		return nil, nil
	}
	if a.sorted {
		sort.SliceStable(a.items, func(i, j int) bool { return a.items[i].key < a.items[j].key })
	}
	parts := make([]string, len(a.items))
	for i, it := range a.items {
		parts[i] = it.val
	}
	return strings.Join(parts, a.sep), nil
}

func (a *concatAgg) Final(*sqlite.FunctionContext) {}
