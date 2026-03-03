package client

import (
	"github.com/kitelabs-io/aptstream/filter"
	v1 "github.com/kitelabs-io/aptstream/grpc/aptos/indexer/v1"
)

func mapRemoteTransactionsFilter(filter *filter.RemoteTransactionFilter) *v1.BooleanTransactionFilter {
	if filter == nil || len(filter.Events) == 0 {
		return nil
	}

	eventFilters := make([]*v1.BooleanTransactionFilter, 0, len(filter.Events))
	for _, event := range filter.Events {
		if event.StructType == nil && event.DataSubstringFilter == nil {
			continue
		}

		eventFilter := v1.EventFilter{}

		if event.StructType != nil {
			eventFilter.StructType = &v1.MoveStructTagFilter{
				Address: event.StructType.Address,
				Module:  event.StructType.Module,
				Name:    event.StructType.Name,
			}
		}

		if event.DataSubstringFilter != nil {
			eventFilter.DataSubstringFilter = event.DataSubstringFilter
		}

		eventFilters = append(eventFilters, &v1.BooleanTransactionFilter{
			Filter: &v1.BooleanTransactionFilter_ApiFilter{
				ApiFilter: &v1.APIFilter{
					Filter: &v1.APIFilter_EventFilter{
						EventFilter: &eventFilter,
					},
				},
			},
		})
	}

	return &v1.BooleanTransactionFilter{
		Filter: &v1.BooleanTransactionFilter_LogicalOr{
			LogicalOr: &v1.LogicalOrFilters{
				Filters: eventFilters,
			},
		},
	}
}
