package filter

type RemoteTransactionFilter struct {
	Events []RemoteEventFilter
}

type RemoteEventFilter struct {
	StructType          *RemoteMoveStructTagFilter
	DataSubstringFilter *string
}

type RemoteMoveStructTagFilter struct {
	Address *string
	Module  *string
	Name    *string
}
