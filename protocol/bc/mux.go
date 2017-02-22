package bc

type mux struct {
	body struct {
		Sources []ValueSource
		Program Program
		ExtHash Hash
	}
}

const typeMux = "mux1"

func (mux) Type() string            { return typeMux }
func (m *mux) Body() interface{}    { return &m.body }
func (m *mux) Witness() interface{} { return nil }

func newMux(sources []ValueSource, program Program) *mux {
	m := new(mux)
	m.body.Sources = sources
	m.body.Program = program
	return m
}
