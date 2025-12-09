# 接口说明

## 约定端点的请求vo对象实现注册验证tag和多语言翻译的接口方法
后端接受到reqVo对象后，判断是否实现了ValidatorApiRegister接口，如果实现了则调用(reqVo.xxx)实现的接口方法完成注册，为接下来的验证提供自定义tag和翻译支持

- 指定接口请求时注册自定义验证Tag及多语言翻译

- 或者：应用启动时集中注册自定义的所有验证tag和翻译，即无需在指定接口的单独注册，具体策略由开发者决定
  - 见 /frame/component/validate/example/ 的示例