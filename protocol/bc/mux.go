package bc

type mux struct {
	body struct {
		Sources []valueSource
	}
}

const typeMux = "mux1"

func (mux) Type() string            { return typeMux }
func (m *mux) Body() interface{}    { return &m.body }
func (m *mux) Witness() interface{} { return nil }

func newMux(sources []valueSource) *mux {
	m := new(mux)
	m.body.Sources = sources
	return m
}
