# Package validate 提供基于 go-playground/validator 的多语言验证器包装器，支持自定义验证规则和错误消息翻译。

该包是应用框架的验证模块，提供统一的数据验证接口和多语言错误提示功能，包括：
- 多语言验证器和翻译器的统一管理
- 结构体字段验证和动态变量验证
- 自定义验证标签的注册和扩展
- 灵活的错误消息格式化（驼峰/蛇形命名）
- 与应用异常系统的无缝集成

## 支持的语言

内置支持以下语言的验证错误消息翻译：
- en: 英文（默认语言）
- zh-cn: 简体中文
- zh-tw: 繁体中文

可通过配置文件或编程方式扩展更多语言支持。

## 配置方式

通过应用配置文件设置支持的语言列表：

	application:
	  validate:
	    langFlags:
	      - "en"
	      - "zh-cn"
	      - "zh-tw"

如果未配置 langFlags，将默认使用英文验证器。

## 基本使用示例

	// 初始化验证器包装器
	validateWrap := validate.NewWrap(config)

	// 结构体验证
	type UserRequest struct {
		Name  string `validate:"required,min=2,max=50"`
		Email string `validate:"required,email"`
		Age   int    `validate:"required,min=18,max=120"`
	}

	req := &UserRequest{Name: "", Email: "invalid", Age: 15}
	lang := "zh-cn"

	if err := validateWrap.GetValidate(lang).Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			// 返回验证异常，字段名使用蛇形命名
			return validateWrap.Errors(validationErrors, lang, true)
		}
	}

## 动态变量验证

	// 验证单个变量
	email := "invalid-email"
	rule := "required,email"

	if err := validateWrap.GetValidate(lang).Var(email, rule); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			return validateWrap.ErrorsVar(validationErrors, "email", lang, true)
		}
	}

## 映射验证

	// 验证 map 数据
	dataMap := map[string]interface{}{
		"price":    -100,
		"currency": "US",  // 应该是 3 位
	}
	ruleMap := map[string]interface{}{
		"price":    "required,min=0",
		"currency": "required,len=3",
	}

	if errsMap := validateWrap.GetValidate(lang).ValidateMap(dataMap, ruleMap); len(errsMap) > 0 {
		return validateWrap.ErrorsMap(errsMap, lang, true)
	}

## 自定义验证标签

	// 定义自定义验证函数
	func hasCourses(fl validator.FieldLevel) bool {
		courses := fl.Field().Interface().([]string)
		return len(courses) > 0
	}

	// 注册自定义标签
	tagRegisters := []validate.RegisterValidatorTagFunc{
		func(vw validate.ValidateWrapper) error {
			for _, v := range vw.GetValidators() {
				if err := v.RegisterValidation("hascourses", hasCourses); err != nil {
					return err
				}
			}
			// 注册翻译消息...
			return nil
		},
	}

	if errs := validateWrap.RegisterCustomTags(tagRegisters); len(errs) > 0 {
		// 处理注册错误
	}

## 错误消息格式化

支持两种字段名格式化风格：
- 驼峰命名（默认）：firstName -> firstName
- 蛇形命名：firstName -> first_name

错误消息结构：

	{
	  "field_name": "Field Name 不能为空",
	  "email": "Email 必须是有效的邮箱地址"
	}

## 并发安全性

ValidateWrapper 实例本身不是并发读写安全的，推荐使用模式：
- 应用启动阶段：初始化和注册所需的语言验证器
- 运行时阶段：只进行读取操作，避免写入操作

各语言的 validator.Validate 实例是并发安全的，可以在多个 goroutine 中同时使用。

## 扩展新语言

要添加新语言支持，需要：
1. 实现 ValidateInitializer 接口:
   type ValidateInitializer func() ValidateRegister
2. 在配置中添加对应的语言标志
3. ValidateRegister.RegisterToWrap 方法将新的语言验证器注册进 Wrap包装器

示例：

	// 实现日文验证器初始化器
	func GetJaValidateInitializer() ValidateInitializer {
	    return func() ValidateRegister {
	        // 返回日文验证器和翻译器的注册逻辑

	        return NewJaValidate()  // 实现参考 validate.GetZhCNValidateInitializer
	    }
	}