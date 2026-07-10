package ledger

import (
	"fmt"
	"sort"
	"sync"

	"github.com/solguardlabs/compassdtl/src/domain"
)

type AccountBalance struct {
	Available int64
	Reserved  int64
}

func (b AccountBalance) Total() int64 {
	return b.Available + b.Reserved
}

func (b AccountBalance) Snapshot(account domain.AccountID, asset domain.AssetID) domain.BalanceSnapshot {
	return domain.BalanceSnapshot{
		Account:   account,
		Asset:     asset,
		Available: b.Available,
		Reserved:  b.Reserved,
	}
}

type Book struct {
	mu       sync.RWMutex
	balances map[domain.AccountID]map[domain.AssetID]*AccountBalance
	entries  []domain.LedgerEntry
	seq      uint64
}

func NewBook() *Book {
	return &Book{
		balances: make(map[domain.AccountID]map[domain.AssetID]*AccountBalance),
		entries:  make([]domain.LedgerEntry, 0, 128),
	}
}

func (b *Book) Clone() *Book {
	b.mu.RLock()
	defer b.mu.RUnlock()
	clone := NewBook()
	clone.seq = b.seq
	for account, byAsset := range b.balances {
		for asset, balance := range byAsset {
			clone.setBalance(account, asset, AccountBalance{
				Available: balance.Available,
				Reserved:  balance.Reserved,
			})
		}
	}
	clone.entries = append(clone.entries, b.entries...)
	return clone
}

func (b *Book) Seed(account domain.AccountID, asset domain.AssetID, available int64) error {
	if available < 0 {
		return domain.Invalid("seed balance cannot be negative")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	current := b.balance(account, asset)
	current.Available = available
	return nil
}

func (b *Book) Credit(account domain.AccountID, asset domain.AssetID, amount int64, memo string, epoch uint64, refs EntryRefs) ([]domain.LedgerEntry, error) {
	if amount < 0 {
		return nil, domain.Invalid("credit amount cannot be negative")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	current := b.balance(account, asset)
	current.Available += amount
	entry := b.entry(domain.EntryCredit, account, asset, amount, current.Available, memo, epoch, refs)
	b.entries = append(b.entries, entry)
	return []domain.LedgerEntry{entry}, nil
}

func (b *Book) Debit(account domain.AccountID, asset domain.AssetID, amount int64, memo string, epoch uint64, refs EntryRefs) ([]domain.LedgerEntry, error) {
	if amount < 0 {
		return nil, domain.Invalid("debit amount cannot be negative")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	current := b.balance(account, asset)
	if current.Available < amount {
		return nil, domain.Insufficient(fmt.Sprintf("account %s has insufficient %s", account, asset))
	}
	current.Available -= amount
	entry := b.entry(domain.EntryDebit, account, asset, amount, current.Available, memo, epoch, refs)
	b.entries = append(b.entries, entry)
	return []domain.LedgerEntry{entry}, nil
}

func (b *Book) Reserve(account domain.AccountID, asset domain.AssetID, amount int64, memo string, epoch uint64, refs EntryRefs) ([]domain.LedgerEntry, error) {
	if amount < 0 {
		return nil, domain.Invalid("reserve amount cannot be negative")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	current := b.balance(account, asset)
	if current.Available < amount {
		return nil, domain.Insufficient(fmt.Sprintf("account %s has insufficient %s", account, asset))
	}
	current.Available -= amount
	current.Reserved += amount
	entry := b.entry(domain.EntryReserve, account, asset, amount, current.Available, memo, epoch, refs)
	b.entries = append(b.entries, entry)
	return []domain.LedgerEntry{entry}, nil
}

func (b *Book) Release(account domain.AccountID, asset domain.AssetID, amount int64, memo string, epoch uint64, refs EntryRefs) ([]domain.LedgerEntry, error) {
	if amount < 0 {
		return nil, domain.Invalid("release amount cannot be negative")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	current := b.balance(account, asset)
	if current.Reserved < amount {
		return nil, domain.Insufficient(fmt.Sprintf("account %s has insufficient reserved %s", account, asset))
	}
	current.Reserved -= amount
	current.Available += amount
	entry := b.entry(domain.EntryRelease, account, asset, amount, current.Available, memo, epoch, refs)
	b.entries = append(b.entries, entry)
	return []domain.LedgerEntry{entry}, nil
}

func (b *Book) ConsumeReserved(account domain.AccountID, asset domain.AssetID, amount int64, memo string, epoch uint64, refs EntryRefs) ([]domain.LedgerEntry, error) {
	if amount < 0 {
		return nil, domain.Invalid("reserved debit amount cannot be negative")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	current := b.balance(account, asset)
	if current.Reserved < amount {
		return nil, domain.Insufficient(fmt.Sprintf("account %s has insufficient reserved %s", account, asset))
	}
	current.Reserved -= amount
	entry := b.entry(domain.EntryDebit, account, asset, amount, current.Available, memo, epoch, refs)
	b.entries = append(b.entries, entry)
	return []domain.LedgerEntry{entry}, nil
}

func (b *Book) Transfer(from domain.AccountID, to domain.AccountID, asset domain.AssetID, amount int64, memo string, epoch uint64, refs EntryRefs) ([]domain.LedgerEntry, error) {
	if amount < 0 {
		return nil, domain.Invalid("transfer amount cannot be negative")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	source := b.balance(from, asset)
	if source.Available < amount {
		return nil, domain.Insufficient(fmt.Sprintf("account %s has insufficient %s", from, asset))
	}
	dest := b.balance(to, asset)
	source.Available -= amount
	dest.Available += amount
	debit := b.entry(domain.EntryDebit, from, asset, amount, source.Available, memo, epoch, refs)
	credit := b.entry(domain.EntryCredit, to, asset, amount, dest.Available, memo, epoch, refs)
	b.entries = append(b.entries, debit, credit)
	return []domain.LedgerEntry{debit, credit}, nil
}

func (b *Book) TransferReserved(from domain.AccountID, to domain.AccountID, sourceAsset domain.AssetID, destAsset domain.AssetID, sourceAmount int64, destAmount int64, memo string, epoch uint64, refs EntryRefs) ([]domain.LedgerEntry, error) {
	if sourceAmount < 0 || destAmount < 0 {
		return nil, domain.Invalid("transfer amount cannot be negative")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	source := b.balance(from, sourceAsset)
	if source.Reserved < sourceAmount {
		return nil, domain.Insufficient(fmt.Sprintf("account %s has insufficient reserved %s", from, sourceAsset))
	}
	dest := b.balance(to, destAsset)
	source.Reserved -= sourceAmount
	dest.Available += destAmount
	debit := b.entry(domain.EntryDebit, from, sourceAsset, sourceAmount, source.Available, memo, epoch, refs)
	credit := b.entry(domain.EntryCredit, to, destAsset, destAmount, dest.Available, memo, epoch, refs)
	b.entries = append(b.entries, debit, credit)
	return []domain.LedgerEntry{debit, credit}, nil
}

func (b *Book) PayFeeFromReserved(from domain.AccountID, to domain.AccountID, asset domain.AssetID, amount int64, memo string, epoch uint64, refs EntryRefs) ([]domain.LedgerEntry, error) {
	if amount < 0 {
		return nil, domain.Invalid("fee amount cannot be negative")
	}
	if amount == 0 {
		return nil, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	source := b.balance(from, asset)
	if source.Reserved < amount {
		return nil, domain.Insufficient(fmt.Sprintf("account %s has insufficient reserved fees %s", from, asset))
	}
	sink := b.balance(to, asset)
	source.Reserved -= amount
	sink.Available += amount
	debit := b.entry(domain.EntryFee, from, asset, amount, source.Available, memo, epoch, refs)
	credit := b.entry(domain.EntryFee, to, asset, amount, sink.Available, memo, epoch, refs)
	b.entries = append(b.entries, debit, credit)
	return []domain.LedgerEntry{debit, credit}, nil
}

func (b *Book) Available(account domain.AccountID, asset domain.AssetID) int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if byAsset, ok := b.balances[account]; ok {
		if balance, ok := byAsset[asset]; ok {
			return balance.Available
		}
	}
	return 0
}

func (b *Book) Reserved(account domain.AccountID, asset domain.AssetID) int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if byAsset, ok := b.balances[account]; ok {
		if balance, ok := byAsset[asset]; ok {
			return balance.Reserved
		}
	}
	return 0
}

func (b *Book) Entries() []domain.LedgerEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]domain.LedgerEntry(nil), b.entries...)
}

func (b *Book) Snapshots() []domain.BalanceSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()
	snapshots := make([]domain.BalanceSnapshot, 0)
	for account, byAsset := range b.balances {
		for asset, balance := range byAsset {
			if balance.Available == 0 && balance.Reserved == 0 {
				continue
			}
			snapshots = append(snapshots, balance.Snapshot(account, asset))
		}
	}
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Account == snapshots[j].Account {
			return snapshots[i].Asset < snapshots[j].Asset
		}
		return snapshots[i].Account < snapshots[j].Account
	})
	return snapshots
}

func (b *Book) AssertSolvent() []domain.AuditIssue {
	b.mu.RLock()
	defer b.mu.RUnlock()
	issues := make([]domain.AuditIssue, 0)
	for account, byAsset := range b.balances {
		for asset, balance := range byAsset {
			if balance.Available < 0 {
				issues = append(issues, domain.AuditIssue{
					Code:     "negative_available",
					Severity: "critical",
					Message:  fmt.Sprintf("%s has negative available %s", account, asset),
				})
			}
			if balance.Reserved < 0 {
				issues = append(issues, domain.AuditIssue{
					Code:     "negative_reserved",
					Severity: "critical",
					Message:  fmt.Sprintf("%s has negative reserved %s", account, asset),
				})
			}
		}
	}
	return issues
}

func (b *Book) balance(account domain.AccountID, asset domain.AssetID) *AccountBalance {
	byAsset, ok := b.balances[account]
	if !ok {
		byAsset = make(map[domain.AssetID]*AccountBalance)
		b.balances[account] = byAsset
	}
	current, ok := byAsset[asset]
	if !ok {
		current = &AccountBalance{}
		byAsset[asset] = current
	}
	return current
}

func (b *Book) setBalance(account domain.AccountID, asset domain.AssetID, balance AccountBalance) {
	byAsset, ok := b.balances[account]
	if !ok {
		byAsset = make(map[domain.AssetID]*AccountBalance)
		b.balances[account] = byAsset
	}
	copied := balance
	byAsset[asset] = &copied
}

func (b *Book) entry(entryType domain.LedgerEntryType, account domain.AccountID, asset domain.AssetID, amount int64, balance int64, memo string, epoch uint64, refs EntryRefs) domain.LedgerEntry {
	b.seq++
	return domain.LedgerEntry{
		ID:       fmt.Sprintf("ledger:%06d", b.seq),
		Type:     entryType,
		Account:  account,
		Asset:    asset,
		Amount:   amount,
		Balance:  balance,
		TicketID: refs.TicketID,
		RouteID:  refs.RouteID,
		Memo:     memo,
		Epoch:    epoch,
	}
}
