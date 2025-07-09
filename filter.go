package aptosstream

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

type booleanTransactionFilter interface {
	match(*transaction.Transaction) bool
}

// ----------------------------- Logical Filters -----------------------------
type Not struct {
	Filter booleanTransactionFilter
}

func (n *Not) match(tx *transaction.Transaction) bool {
	return !n.Filter.match(tx)
}

type And struct {
	Filters []booleanTransactionFilter
}

func (a *And) match(tx *transaction.Transaction) bool {
	for _, filter := range a.Filters {
		if !filter.match(tx) {
			return false
		}
	}
	return true
}

type Or struct {
	Filters []booleanTransactionFilter
}

func (o *Or) match(tx *transaction.Transaction) bool {
	for _, filter := range o.Filters {
		if filter.match(tx) {
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

func (t *TransactionRootFilter) match(tx *transaction.Transaction) bool {
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

func (u *UserTransactionFilter) match(tx *transaction.Transaction) bool {
	// type check if txn is user transaction
	userTransaction := tx.GetUser()
	if userTransaction == nil {
		return false
	}

	if u.Sender != nil && userTransaction.GetRequest().GetSender() != *u.Sender {
		return false
	}
	if u.PayloadFilter != nil {
		return u.PayloadFilter.match(userTransaction.GetRequest().GetPayload())
	}
	return true
}

type UserTransactionPayloadFilter struct {
	EntryFunctionFilter *EntryFunctionFilter
}

func (u *UserTransactionPayloadFilter) match(payload *transaction.TransactionPayload) bool {
	if payload == nil {
		return false
	}
	if u.EntryFunctionFilter != nil {
		return u.EntryFunctionFilter.match(payload.GetEntryFunctionPayload())
	}
	return true
}

type EntryFunctionFilter struct {
	Address    *string
	ModuleName *string
	Function   *string
}

func (e *EntryFunctionFilter) match(payload *transaction.EntryFunctionPayload) bool {
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

func (e *EventFilter) match(tx *transaction.Transaction) bool {
	// type check if txn is user transaction
	userTransaction := tx.GetUser()
	if userTransaction == nil {
		// event filter is not applicable to non-user transactions
		return false
	}
	if e.StructType != nil {
		for _, event := range userTransaction.GetEvents() {
			if e.StructType.match(event.GetType().GetStruct()) {
				return true
			}
		}
		return false
	}

	if e.DataSubstringFilter != nil {
		for _, event := range userTransaction.GetEvents() {
			if strings.Contains(event.GetData(), *e.DataSubstringFilter) {
				return true
			}
		}
		return false
	}

	return true
}

type MoveStructTagFilter struct {
	Address *string
	Module  *string
	Name    *string
}

func (m *MoveStructTagFilter) match(event *transaction.MoveStructTag) bool {
	if event == nil {
		return false
	}

	if m.Address != nil && event.GetAddress() != *m.Address {
		return false
	}

	if m.Module != nil && event.GetModule() != *m.Module {
		return false
	}

	if m.Name != nil && event.GetName() != *m.Name {
		return false
	}

	return true
}
