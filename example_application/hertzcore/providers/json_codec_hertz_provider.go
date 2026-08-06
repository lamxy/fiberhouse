package providers

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/constant"
	hertzconst "github.com/lamxy/fiberhouse/example_application/hertzcore/constant"
)

// SonicJCodecHertzProvider hertz 的 sonic JSON 編解碼提供者
//
// 框架的 JsonCodecPManager 同時以 Version()==BootConfig.TrafficCodec 與
// Target()==BootConfig.CoreType 兩個條件選取提供者，故每個核心需各自註冊編解碼提供者。
type SonicJCodecHertzProvider struct {
	fiberhouse.IProvider
	jcodec fiberhouse.JsonWrapper
}

// NewSonicJCodecHertzProvider 建立 hertz 的 sonic 編解碼提供者
func NewSonicJCodecHertzProvider() *SonicJCodecHertzProvider {
	return &SonicJCodecHertzProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("SonicJCodecHertzProvider").
			SetVersion(constant.TrafficCodecWithSonic).
			SetTarget(hertzconst.CoreTypeWithHertz).
			SetType(fiberhouse.ProviderTypeDefault().GroupTrafficCodecChoose),
	}
}

// Initialize 取得全域已註冊的預設編解碼器實例
func (j *SonicJCodecHertzProvider) Initialize(ctx fiberhouse.IContext, fn ...fiberhouse.ProviderInitFunc) (any, error) {
	if j.Status() == fiberhouse.StateLoaded && j.jcodec != nil {
		return j.jcodec, nil
	}
	jcodec, err := fiberhouse.GetInstance[fiberhouse.JsonWrapper](
		ctx.GetStarter().GetApplication().GetDefaultTrafficCodecKey())
	if err != nil {
		return nil, err
	}
	j.jcodec = jcodec
	return j.jcodec, nil
}

// StdJCodecHertzProvider hertz 的標準庫 JSON 編解碼提供者
type StdJCodecHertzProvider struct {
	fiberhouse.IProvider
	jcodec fiberhouse.JsonWrapper
}

// NewStdJCodecHertzProvider 建立 hertz 的標準庫編解碼提供者
func NewStdJCodecHertzProvider() *StdJCodecHertzProvider {
	return &StdJCodecHertzProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("StdJCodecHertzProvider").
			SetVersion(constant.TrafficCodecWithStd).
			SetTarget(hertzconst.CoreTypeWithHertz).
			SetType(fiberhouse.ProviderTypeDefault().GroupTrafficCodecChoose),
	}
}

// Initialize 取得全域已註冊的預設編解碼器實例
func (j *StdJCodecHertzProvider) Initialize(ctx fiberhouse.IContext, fn ...fiberhouse.ProviderInitFunc) (any, error) {
	if j.Status() == fiberhouse.StateLoaded && j.jcodec != nil {
		return j.jcodec, nil
	}
	jcodec, err := fiberhouse.GetInstance[fiberhouse.JsonWrapper](
		ctx.GetStarter().GetApplication().GetDefaultTrafficCodecKey())
	if err != nil {
		return nil, err
	}
	j.jcodec = jcodec
	return j.jcodec, nil
}
