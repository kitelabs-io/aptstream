package filter

// Filter behaviors:
// - A nil filter for any field means that criterion is ignored (matches all).
// - Logical 'And' with no sub-filters returns true.
// - Logical 'Or' with no sub-filters returns false.

import (
	"strings"

	transaction "github.com/kitelabs-io/aptstream/grpc/aptos/transaction/v1"
)

type TransactionType int32

const (
	TransactionTypeUnspecified TransactionType = iota
	TransactionTypeGenesis
	TransactionTypeBlockMetadata
	TransactionTypeStateCheckpoint
	TransactionTypeUser
	// values 5-19 skipped for no reason
	TransactionTypeValidator     = 20
	TransactionTypeBlockEpilogue = 21
)

// Filter provides a predicate interface for filtering aptos transactions.
type Filter interface {
	Match(*transaction.Transaction) bool
}

// ----------------------------- Logical Filters -----------------------------
type Not struct {
	Filter Filter
}

func (n *Not) Match(tx *transaction.Transaction) bool {
	return !n.Filter.Match(tx)
}

type And struct {
	Filters []Filter
}

// if Filters is empty, it will return true
func (a *And) Match(tx *transaction.Transaction) bool {
	for _, filter := range a.Filters {
		if !filter.Match(tx) {
			return false
		}
	}
	return true
}

type Or struct {
	Filters []Filter
}

// if Filters is empty, it will return false
func (o *Or) Match(tx *transaction.Transaction) bool {
	for _, filter := range o.Filters {
		if filter.Match(tx) {
			return true
		}
	}
	return false
}

// ----------------------------- Transaction Root Filter -----------------------------
type TransactionRootFilter struct {
	Success *bool
	TxnType *TransactionType
}

func (t *TransactionRootFilter) Match(tx *transaction.Transaction) bool {
	if t.Success != nil && tx.GetInfo().GetSuccess() != *t.Success {
		return false
	}
	if t.TxnType != nil && TransactionType(tx.GetType()) != *t.TxnType {
		return false
	}
	return true
}

// ----------------------------- User Transaction Filter -----------------------------
type UserTransactionFilter struct {
	Sender        *string
	PayloadFilter *UserTransactionPayloadFilter
}

func (u *UserTransactionFilter) Match(tx *transaction.Transaction) bool {
	// type check if txn is user transaction
	userTransaction := tx.GetUser()
	if userTransaction == nil {
		return false
	}

	if u.Sender != nil && userTransaction.GetRequest().GetSender() != *u.Sender {
		return false
	}
	if u.PayloadFilter != nil {
		return u.PayloadFilter.Match(userTransaction.GetRequest().GetPayload())
	}
	return true
}

type UserTransactionPayloadFilter struct {
	EntryFunctionFilter *EntryFunctionFilter
}

func (u *UserTransactionPayloadFilter) Match(payload *transaction.TransactionPayload) bool {
	if payload == nil {
		return false
	}
	if u.EntryFunctionFilter != nil {
		return u.EntryFunctionFilter.Match(payload.GetEntryFunctionPayload())
	}
	return true
}

type EntryFunctionFilter struct {
	Address    *string
	ModuleName *string
	Function   *string
}

func (e *EntryFunctionFilter) Match(payload *transaction.EntryFunctionPayload) bool {
	// type check if payload is entry function payload
	if payload == nil {
		return false
	}

	if e.Address != nil && payload.GetFunction().GetModule().GetAddress() != *e.Address {
		return false
	}
	if e.ModuleName != nil && payload.GetFunction().GetModule().GetName() != *e.ModuleName {
		return false
	}
	if e.Function != nil && payload.GetFunction().GetName() != *e.Function {
		return false
	}
	return true
}

// ----------------------------- Event Filter -----------------------------
type EventFilter struct {
	StructType          *MoveStructTagFilter
	DataSubstringFilter *string
}

// match returns true if any of the events in the transaction match the filter criteria.
// If both StructType and DataSubstringFilter are nil, it matches any transaction that has at least one event.
func (e *EventFilter) Match(tx *transaction.Transaction) bool {
	userTransaction := tx.GetUser()
	if userTransaction == nil {
		// event filter is not applicable to non-user transactions
		return false
	}

	// no filters means accept all events
	if e.StructType == nil && e.DataSubstringFilter == nil {
		return true
	}

	for _, event := range userTransaction.GetEvents() {
		// If a filter is nil, it's a match for that part.
		structMatches := e.StructType == nil || e.StructType.Match(event.GetType().GetStruct())
		dataMatches := e.DataSubstringFilter == nil || strings.Contains(event.GetData(), *e.DataSubstringFilter)

		if structMatches && dataMatches {
			return true
		}
	}

	return false
}

type MoveStructTagFilter struct {
	Address *string
	Module  *string
	Name    *string
}

func (m *MoveStructTagFilter) Match(structTag *transaction.MoveStructTag) bool {
	if structTag == nil {
		return false
	}

	if m.Address != nil && structTag.GetAddress() != *m.Address {
		return false
	}

	if m.Module != nil && structTag.GetModule() != *m.Module {
		return false
	}

	if m.Name != nil && structTag.GetName() != *m.Name {
		return false
	}

	return true
}
