package filter

import v1 "github.com/kitelabs-io/aptstream/grpc/aptos/indexer/v1"

type isRemoteBooleanTransactionFilter interface {
	isRemoteBooleanTransactionFilter()
}

type RemoteBooleanTransactionFilter struct {
	Filter isRemoteBooleanTransactionFilter
}

func (*RemoteBooleanTransactionFilter) isRemoteBooleanTransactionFilter() {}

type RemoteLogicalOrFilters struct {
	Filters []*RemoteBooleanTransactionFilter
}

func (*RemoteLogicalOrFilters) isRemoteBooleanTransactionFilter() {}

type RemoteTransactionFilter struct {
	Filter isRemoteTransactionFilter
}

func (*RemoteTransactionFilter) isRemoteBooleanTransactionFilter() {}

type isRemoteTransactionFilter interface {
	isRemoteTransactionFilter()
}

type RemoteEventFilter struct {
	StructType          *RemoteMoveStructTagFilter
	DataSubstringFilter *string
}

func (*RemoteEventFilter) isRemoteTransactionFilter() {}

type RemoteMoveStructTagFilter struct {
	Address *string
	Module  *string
	Name    *string
}

func (f *RemoteBooleanTransactionFilter) ToRemoteTransactionsFilter() *v1.BooleanTransactionFilter {
	if f == nil || f.Filter == nil {
		return nil
	}

	return mapRemoteBooleanTransactionFilter(f.Filter)
}

func mapRemoteBooleanTransactionFilter(filter isRemoteBooleanTransactionFilter) *v1.BooleanTransactionFilter {
	if filter == nil {
		return nil
	}

	switch f := filter.(type) {
	case *RemoteBooleanTransactionFilter:
		return mapRemoteBooleanTransactionFilter(f.Filter)
	case *RemoteLogicalOrFilters:
		orFilters := make([]*v1.BooleanTransactionFilter, 0, len(f.Filters))
		for _, filter := range f.Filters {
			mappedFilter := mapRemoteBooleanTransactionFilter(filter)
			if mappedFilter != nil {
				orFilters = append(orFilters, mappedFilter)
			}
		}
		return &v1.BooleanTransactionFilter{
			Filter: &v1.BooleanTransactionFilter_LogicalOr{
				LogicalOr: &v1.LogicalOrFilters{
					Filters: orFilters,
				},
			},
		}
	case *RemoteTransactionFilter:
		apiFilter := mapRemoteTransactionFilterToApiFilter(f)
		return &v1.BooleanTransactionFilter{
			Filter: apiFilter,
		}
	default:
		return nil
	}
}

func mapRemoteTransactionFilterToApiFilter(filter *RemoteTransactionFilter) *v1.BooleanTransactionFilter_ApiFilter {
	if filter == nil || filter.Filter == nil {
		return nil
	}

	switch f := filter.Filter.(type) {
	case *RemoteEventFilter:
		eventFilter := v1.EventFilter{}
		if f.StructType != nil {
			eventFilter.StructType = &v1.MoveStructTagFilter{
				Address: f.StructType.Address,
				Module:  f.StructType.Module,
				Name:    f.StructType.Name,
			}
		}
		if f.DataSubstringFilter != nil {
			eventFilter.DataSubstringFilter = f.DataSubstringFilter
		}
		return &v1.BooleanTransactionFilter_ApiFilter{
			ApiFilter: &v1.APIFilter{
				Filter: &v1.APIFilter_EventFilter{
					EventFilter: &eventFilter,
				},
			},
		}
	default:
		return nil
	}
}
