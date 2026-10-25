package game

import "errors"

// SealedBids is private persisted state. Never embed it in a public Match/View.
// Power is not debited until everyone has committed: otherwise public balances
// would disclose bids before the simultaneous reveal.
type SealedBids struct {
	Participants []string       `json:"participants"`
	Values       map[string]int `json:"values"`
}

type BidsView struct {
	Submitted map[string]bool `json:"submitted"`
	YourBid   *int            `json:"yourBid,omitempty"`
	Values    map[string]int  `json:"values,omitempty"`
}

func newSealedBids(houses []string) SealedBids {
	return SealedBids{Participants: append([]string{}, houses...), Values: map[string]int{}}
}

func (b *SealedBids) submit(house string, amount, available int) error {
	participant := false
	for _, h := range b.Participants {
		if house == h {
			participant = true
			break
		}
	}
	if !participant {
		return errors.New("Вы не участвуете в этих торгах")
	}
	if _, submitted := b.Values[house]; submitted {
		return errors.New("Ставка уже подтверждена")
	}
	if amount < 0 || amount > available {
		return errors.New("Ставка должна быть от нуля до доступной власти")
	}
	b.Values[house] = amount
	return nil
}

func (b *SealedBids) complete() bool {
	if len(b.Participants) == 0 {
		return false
	}
	for _, house := range b.Participants {
		if _, ok := b.Values[house]; !ok {
			return false
		}
	}
	return true
}

func (b *SealedBids) view(house string) BidsView {
	v := BidsView{Submitted: map[string]bool{}}
	for _, h := range b.Participants {
		_, v.Submitted[h] = b.Values[h]
	}
	if amount, ok := b.Values[house]; ok {
		v.YourBid = &amount
	}
	if b.complete() {
		v.Values = copyBids(b.Values)
	}
	return v
}

func copyBids(source map[string]int) map[string]int {
	result := make(map[string]int, len(source))
	for house, amount := range source {
		result[house] = amount
	}
	return result
}
