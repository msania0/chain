package txbuilder

import (
	"context"
	stdjson "encoding/json"

	"chain/crypto/sha3pool"
	"chain/encoding/json"
	"chain/protocol/bc"
	"chain/protocol/vm"
	"chain/protocol/vmutil"
)

var retirementProgram = vmutil.NewBuilder().AddOp(vm.OP_FAIL).Program

func DecodeControlReceiverAction(data []byte) (Action, error) {
	a := new(controlReceiverAction)
	err := stdjson.Unmarshal(data, a)
	return a, err
}

type controlReceiverAction struct {
	bc.AssetAmount
	Receiver      *Receiver `json:"receiver"`
	ReferenceData json.Map  `json:"reference_data"`
}

func (a *controlReceiverAction) Build(ctx context.Context, b *TemplateBuilder) error {
	var missing []string
	if a.Receiver == nil {
		missing = append(missing, "receiver")
	} else {
		if len(a.Receiver.ControlProgram) == 0 {
			missing = append(missing, "receiver.control_program")
		}
		if a.Receiver.ExpiresAt.IsZero() {
			missing = append(missing, "receiver.expires_at")
		}
	}
	if a.AssetID == (bc.AssetID{}) {
		missing = append(missing, "asset_id")
	}
	if len(missing) > 0 {
		return MissingFieldsError(missing...)
	}

	b.RestrictMaxTime(a.Receiver.ExpiresAt)
	var refdataHash bc.Hash
	if len(a.ReferenceData) > 0 {
		sha3pool.Sum256(refdataHash[:], a.ReferenceData)
	}
	return b.AddOutput(a.AssetAmount, bc.Program{VMVersion: 1, Code: a.Receiver.ControlProgram}, refdataHash)
}

func DecodeControlProgramAction(data []byte) (Action, error) {
	a := new(controlProgramAction)
	err := stdjson.Unmarshal(data, a)
	return a, err
}

type controlProgramAction struct {
	bc.AssetAmount
	Program       json.HexBytes `json:"control_program"`
	ReferenceData json.Map      `json:"reference_data"`
}

func (a *controlProgramAction) Build(ctx context.Context, b *TemplateBuilder) error {
	var missing []string
	if len(a.Program) == 0 {
		missing = append(missing, "control_program")
	}
	if a.AssetID == (bc.AssetID{}) {
		missing = append(missing, "asset_id")
	}
	if len(missing) > 0 {
		return MissingFieldsError(missing...)
	}
	var refdataHash bc.Hash
	if len(a.ReferenceData) > 0 {
		sha3pool.Sum256(refdataHash[:], a.ReferenceData)
	}
	return b.AddOutput(a.AssetAmount, bc.Program{VMVersion: 1, Code: a.Program}, refdataHash)
}

func DecodeSetTxRefDataAction(data []byte) (Action, error) {
	a := new(setTxRefDataAction)
	err := stdjson.Unmarshal(data, a)
	return a, err
}

type setTxRefDataAction struct {
	Data json.Map `json:"reference_data"`
}

func (a *setTxRefDataAction) Build(ctx context.Context, b *TemplateBuilder) error {
	if len(a.Data) == 0 {
		return MissingFieldsError("reference_data")
	}
	return b.setReferenceData(a.Data)
}

func DecodeRetireAction(data []byte) (Action, error) {
	a := new(retireAction)
	err := stdjson.Unmarshal(data, a)
	return a, err
}

type retireAction struct {
	bc.AssetAmount
	ReferenceData json.Map `json:"reference_data"`
}

func (a *retireAction) Build(ctx context.Context, b *TemplateBuilder) error {
	var missing []string
	if a.AssetID == (bc.AssetID{}) {
		missing = append(missing, "asset_id")
	}
	if a.Amount == 0 {
		missing = append(missing, "amount")
	}
	if len(missing) > 0 {
		return MissingFieldsError(missing...)
	}
	var refdataHash bc.Hash
	if len(a.ReferenceData) > 0 {
		sha3pool.Sum256(refdataHash[:], a.ReferenceData)
	}
	return b.AddRetirement(a.AssetAmount, refdataHash)
}
