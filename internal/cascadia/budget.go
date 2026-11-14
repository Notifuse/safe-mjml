package cascadia

// Budget bounds the work of matching the selectors parsed with it
// (ParseWithBudget). Each element a selector tests, each attribute list it
// reads, each node it visits (an ancestor, a sibling, a descendant for :has
// or :contains), each :not(), every bytesPerStep bytes of a value or a text
// it reads and each byte it lowers the case of cost a step. Once the steps are spent, every selector
// fails at once and Exceeded reports it: a stylesheet and a document chosen
// to make matching slow cost a bounded time. A Budget is for one goroutine.
type Budget struct {
	left int64
}

// bytesPerStep is how many bytes of an attribute value or a text a selector
// reads for one step, when it copies, compares or searches them. Lowering
// their case costs a step for each byte more (about 2.5 ns a byte: 20,000
// tests of [title*=x i] on a 500 KB title took 25 s at 64 bytes a step).
const bytesPerStep = 16

// NewBudget returns a Budget of steps.
func NewBudget(steps int64) *Budget { return &Budget{left: steps} }

// Spend takes n steps and reports whether the Budget had them. A nil Budget
// has no limit.
func (b *Budget) Spend(n int) bool {
	if b == nil {
		return true
	}
	b.left -= int64(n)
	return b.left >= 0
}

// Exceeded reports whether more steps were taken than the Budget had.
func (b *Budget) Exceeded() bool { return b != nil && b.left < 0 }
