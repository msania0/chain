package bc

type Spend struct {
	body struct {
		SpentOutput *EntryRef
		Data        Hash
		ExtHash     Hash
	}
	witness struct {
		Destination valueDestination
		Arguments   [][]byte
		ExtHash     Hash
	}
}

const typeSpend = "spend1"

func (Spend) Type() string            { return typeSpend }
func (s *Spend) Body() interface{}    { return &s.body }
func (s *Spend) Witness() interface{} { return &s.witness }

func (s *Spend) SpentOutput() *EntryRef {
	return s.body.SpentOutput
}

func (s *Spend) Data() Hash {
	return s.body.Data
}

func (s *Spend) Arguments() [][]byte {
	return s.witness.Arguments
}

func (s *Spend) SetArguments(args [][]byte) {
	s.witness.Arguments = args
}

func NewSpend(spentOutput *EntryRef, data Hash) *Spend {
	s := new(Spend)
	s.body.SpentOutput = spentOutput
	s.body.Data = data
	return s
}
