package bc

type Output struct {
	body struct {
		Source         ValueSource
		ControlProgram Program
		Data           Hash
		ExtHash        Hash
	}
}

const typeOutput = "output1"

func (Output) Type() string            { return typeOutput }
func (o *Output) Body() interface{}    { return &o.body }
func (o *Output) Witness() interface{} { return nil }

func (o *Output) AssetID() AssetID {
	return o.body.Source.Value.AssetID
}

func (o *Output) Amount() uint64 {
	return o.body.Source.Value.Amount
}

func (o *Output) ControlProgram() Program {
	return o.body.ControlProgram
}

func (o *Output) Data() Hash {
	return o.body.Data
}

func NewOutput(source ValueSource, controlProgram Program, data Hash) *Output {
	out := new(Output)
	out.body.Source = source
	out.body.ControlProgram = controlProgram
	out.body.Data = data
	return out
}
