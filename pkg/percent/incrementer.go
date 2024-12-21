package percent

type Incrementer struct {
	DoneIncrements int
	MaxIncrements  int
}

func (it *Incrementer) IncrementSection(err error) {
	if int64(it.DoneIncrements)+int64(1) > int64(it.MaxIncrements) {
		return
	}
	it.DoneIncrements += 1
}

func (it *Incrementer) CurrentStep() int {
	return it.DoneIncrements
}
