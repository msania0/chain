package bc

type Nonce struct {
	body struct {
		Program   Program
		TimeRange *EntryRef
		ExtHash   Hash
	}
}

const typeNonce = "nonce1"

func (Nonce) Type() string            { return typeNonce }
func (n *Nonce) Body() interface{}    { return &n.body }
func (n *Nonce) Witness() interface{} { return nil }

func (n *Nonce) TimeRange() *EntryRef {
	return n.body.TimeRange
}

func newNonce(p Program, tr *EntryRef) *Nonce {
	n := new(Nonce)
	n.body.Program = p
	n.body.TimeRange = tr
	return n
}
