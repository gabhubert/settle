// OWNER: stan

package endpoint

import (
	"fmt"
	"math/big"
	"net/http"

	"github.com/spolu/settle/lib/db"
	"github.com/spolu/settle/lib/env"
	"github.com/spolu/settle/lib/errors"
	"github.com/spolu/settle/lib/format"
	"github.com/spolu/settle/lib/ptr"
	"github.com/spolu/settle/lib/svc"
	"github.com/spolu/settle/mint"
	"github.com/spolu/settle/mint/lib/authentication"
	"github.com/spolu/settle/mint/model"
)

const (
	// EndPtCreateOffer creates a new offer.
	EndPtCreateOffer EndPtName = "CreateOffer"
)

func init() {
	registrar[EndPtCreateOffer] = NewCreateOffer
}

// CreateOffer creates a new canonical offer and triggers its propagation to
// all the mints involved.
type CreateOffer struct {
	Client *mint.Client

	Owner      string
	Pair       []mint.AssetResource
	BasePrice  big.Int
	QuotePrice big.Int
	Amount     big.Int
}

// NewCreateOffer constructs and initialiezes the endpoint.
func NewCreateOffer(
	r *http.Request,
) (Endpoint, error) {
	ctx := r.Context()

	client := &mint.Client{}
	err := client.Init(ctx)
	if err != nil {
		return nil, errors.Trace(err) // 500
	}
	return &CreateOffer{
		Client: client,
	}, nil
}

// Validate validates the input parameters.
func (e *CreateOffer) Validate(
	r *http.Request,
) error {
	ctx := r.Context()

	e.Owner = fmt.Sprintf("%s@%s",
		authentication.Get(ctx).User.Username,
		env.Get(ctx).Config[mint.EnvCfgMintHost])

	// Validate asset pair.
	pair, err := mint.AssetResourcesFromPair(ctx, r.PostFormValue("pair"))
	if err != nil {
		return errors.Trace(errors.NewUserErrorf(err,
			400, "pair_invalid",
			"The asset pair you provided is invalid: %s.",
			r.PostFormValue("pair"),
		))
	}
	e.Pair = pair

	// Validate price.
	basePrice, quotePrice, err := ValidatePrice(r)
	if err != nil {
		return errors.Trace(err) // 400
	}
	e.BasePrice = *basePrice
	e.QuotePrice = *quotePrice

	// Validate amount.
	amount, err := ValidateAmount(r)
	if err != nil {
		return errors.Trace(err) // 400
	}
	e.Amount = *amount

	return nil
}

// Execute executes the endpoint.
func (e *CreateOffer) Execute(
	r *http.Request,
) (*int, *svc.Resp, error) {
	ctx := r.Context()

	ctx = db.Begin(ctx)
	defer db.LoggedRollback(ctx)

	// Create canonical offer locally.
	offer, err := model.CreateCanonicalOffer(ctx,
		authentication.Get(ctx).User.Token,
		e.Owner,
		e.Pair[0].Name, e.Pair[1].Name,
		model.Amount(e.BasePrice), model.Amount(e.QuotePrice),
		model.Amount(e.Amount), model.OfStActive)
	if err != nil {
		return nil, nil, errors.Trace(err) // 500
	}

	db.Commit(ctx)

	// TODO(stan): propagation

	return ptr.Int(http.StatusCreated), &svc.Resp{
		"offer": format.JSONPtr(mint.NewOfferResource(ctx, offer)),
	}, nil
}
