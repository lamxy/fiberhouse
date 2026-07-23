package fiberhouse

import "errors"

// loadProviderManagersAtLocation 按执行位点加载管理器；扩展替代管理器只替代同一位点的普通管理器。
func loadProviderManagersAtLocation(
	managers []IProviderManager,
	location IProviderLocation,
	dependency any,
) (handled bool, replaced bool, err error) {
	var defaults []IProviderManager
	var replacements []IProviderManager

	for _, manager := range managers {
		if manager == nil || manager.Location() == nil ||
			manager.Location().GetLocationID() != location.GetLocationID() {
			continue
		}
		if manager.Type().GetTypeID() == ProviderTypeDefault().GroupExtendReplace.GetTypeID() {
			replacements = append(replacements, manager)
			continue
		}
		defaults = append(defaults, manager)
	}

	selected := defaults
	if len(replacements) > 0 {
		selected = replacements
		replaced = true
	}
	if len(selected) == 0 {
		return false, replaced, nil
	}

	var errs []error
	for _, manager := range selected {
		_, loadErr := manager.LoadProvider(func(IProviderManager) (any, error) {
			return dependency, nil
		})
		if loadErr != nil {
			errs = append(errs, loadErr)
		}
	}
	return true, replaced, errors.Join(errs...)
}
