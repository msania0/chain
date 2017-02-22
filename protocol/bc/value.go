package bc

type ValueSource struct {
	Ref      *EntryRef
	Value    AssetAmount
	Position uint64 // zero unless Ref is a mux
}

type valueDestination struct {
	Ref      *EntryRef
	Position uint64
}
