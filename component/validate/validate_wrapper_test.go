package validate

import (
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newValidateTestWrap(langs ...string) *Wrap {
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"application.validate.langFlags": langs,
	})
	return NewWrap(cfg)
}

func installValidateTestExceptions(t *testing.T) {
	t.Helper()
	manager := globalmanager.NewGlobalManagerOnce()
	key := constant.RegisterKeyPrefix + "exceptions"
	wasRegistered := manager.IsRegistered(key)
	var previous interface{}
	if wasRegistered {
		previous, _ = manager.Get(key)
	}
	manager.Clear(key)
	require.True(t, manager.Register(key, func() (interface{}, error) {
		return exception.ExceptionMap{"InputParamError": {Code: 4000, Msg: "invalid input"}}, nil
	}))
	t.Cleanup(func() {
		manager.Clear(key)
		if wasRegistered {
			manager.Register(key, func() (interface{}, error) { return previous, nil })
		}
	})
}

func TestValidate_LanguagesAndUnsupportedFallback(t *testing.T) {
	w := newValidateTestWrap(LangEn, LangZhCN, LangZhTW, "unsupported")
	assert.ElementsMatch(t, []LangFlag{LangEn, LangZhCN, LangZhTW}, w.GetLangList())
	assert.Len(t, w.GetValidators(), 3)
	assert.Len(t, w.GetTranslators(), 3)
	assert.Same(t, w.GetValidate(LangEn), w.GetValidate("unsupported"))
	assert.Same(t, w.GetTranslator(LangEn), w.GetTranslator("unsupported"))
	assert.NotNil(t, GetEnValidateInitializer()())
	assert.NotNil(t, GetZhCNValidateInitializer()())
	assert.NotNil(t, GetZhTWValidateInitializer()())
	assert.Equal(t, LangEn, GetDefaultLang())
}

func TestValidate_StructVarAndMapTranslations(t *testing.T) {
	installValidateTestExceptions(t)
	w := newValidateTestWrap(LangEn, LangZhCN, LangZhTW)
	type request struct {
		UserName string `validate:"required"`
	}

	for _, lang := range []LangFlag{LangEn, LangZhCN, LangZhTW} {
		err := w.GetValidate(lang).Struct(request{})
		var validationErrors validator.ValidationErrors
		require.ErrorAs(t, err, &validationErrors)
		translated := w.Errors(validationErrors, lang, true)
		data, ok := translated.Data.(map[string]string)
		require.True(t, ok)
		assert.Contains(t, data, "user_name")
		assert.NotEmpty(t, data["user_name"])
		translated.Release()

		err = w.GetValidate(lang).Var("", "required")
		require.ErrorAs(t, err, &validationErrors)
		translated = w.ErrorsVar(validationErrors, "DisplayName", lang, true)
		data, ok = translated.Data.(map[string]string)
		require.True(t, ok)
		assert.NotEmpty(t, data)
		translated.Release()

		mapErrors := w.GetValidate(lang).ValidateMap(
			map[string]interface{}{"PostalCode": ""},
			map[string]interface{}{"PostalCode": "required"},
		)
		translated = w.ErrorsMap(mapErrors, lang, true)
		mapData, ok := translated.Data.(map[string]interface{})
		require.True(t, ok)
		assert.NotEmpty(t, mapData)
		translated.Release()
	}
}

func TestValidate_RegisterCustomTagsAggregatesErrorsAndDuplicateRegistration(t *testing.T) {
	w := newValidateTestWrap()
	first := errors.New("first registration")
	second := errors.New("duplicate registration")
	calls := 0
	errs := w.RegisterCustomTags([]RegisterValidatorTagFunc{
		func(*Wrap) error { calls++; return nil },
		func(*Wrap) error { calls++; return first },
		func(*Wrap) error { calls++; return second },
	})
	assert.Equal(t, 3, calls)
	require.Len(t, errs, 2)
	assert.ErrorIs(t, errs[0], first)
	assert.ErrorIs(t, errs[1], second)
	assert.Nil(t, w.RegisterCustomTags(nil))
}

func TestValidate_FreshInstancesDoNotShareRegistrationState(t *testing.T) {
	first := newValidateTestWrap(LangEn)
	second := newValidateTestWrap(LangEn)
	custom := validator.New()
	first.RegisterValidator("custom", custom)
	first.RegisterTranslator("custom", first.GetTranslator())
	first.RegisterLangFlag("custom")
	assert.Same(t, custom, first.GetValidate("custom"))
	assert.NotContains(t, second.GetLangList(), "custom")
	assert.NotContains(t, second.GetValidators(), "custom")
}
