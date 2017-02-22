package bc

type Issuance struct {
	body struct {
		Anchor  *EntryRef
		Value   AssetAmount
		Data    Hash
		ExtHash Hash
	}
	witness struct {
		Destination         valueDestination
		InitialBlockID      Hash
		AssetDefinitionHash Hash
		IssuanceProgram     Program
		Arguments           [][]byte
		ExtHash             Hash
	}
}

const typeIssuance = "issuance1"

func (Issuance) Type() string              { return typeIssuance }
func (iss *Issuance) Body() interface{}    { return &iss.body }
func (iss *Issuance) Witness() interface{} { return &iss.witness }

func (iss *Issuance) AssetID() AssetID {
	return iss.body.Value.AssetID
}

func (iss *Issuance) Amount() uint64 {
	return iss.body.Value.Amount
}

func (iss *Issuance) Anchor() *EntryRef {
	return iss.body.Anchor
}

func (iss *Issuance) Data() Hash {
	return iss.body.Data
}

func (iss *Issuance) InitialBlockID() Hash {
	return iss.witness.InitialBlockID
}

func (iss *Issuance) AssetDefinitionHash() Hash {
	return iss.witness.AssetDefinitionHash
}

func (iss *Issuance) IssuanceProgram() Program {
	return iss.witness.IssuanceProgram
}

func (iss *Issuance) Arguments() [][]byte {
	return iss.witness.Arguments
}

func (iss *Issuance) SetArguments(args [][]byte) {
	iss.witness.Arguments = args
}

func newIssuance(anchor *EntryRef, value AssetAmount, data Hash) *Issuance {
	iss := new(Issuance)
	iss.body.Anchor = anchor
	iss.body.Value = value
	iss.body.Data = data
	return iss
}
