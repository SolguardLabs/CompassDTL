package ledger

import "github.com/solguardlabs/compassdtl/src/domain"

type EntryRefs struct {
	TicketID domain.TicketID
	RouteID  domain.RouteID
}

func Refs(ticketID domain.TicketID, routeID domain.RouteID) EntryRefs {
	return EntryRefs{TicketID: ticketID, RouteID: routeID}
}

func NoRefs() EntryRefs {
	return EntryRefs{}
}
