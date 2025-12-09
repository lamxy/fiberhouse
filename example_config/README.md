# 设置环境变量
### 由应用启动时的环境变量（应用的类型和应用环境），决定加载哪个配置文件
- 应用类型appType: web、cmd
- 应用环境env: test、dev、prod
- 配置文件名: application_[web|cmd]_[test|dev|prod].yml

### 通过设置引导类型的环境变量，设置应用类型和应用环境，加载具体的配置文件
如：应用类型为web，应用环境为test，则加载配置文件 application_web_test.yml
  - 变量名格式: 前缀APP_ENV_ + yml配置文件配置项路径(用_连接，大小写保持跟yml配置一致)
  - 映射值
    - application_env => application.env 环境： test、prod、dev
    - application_appType => application.appType 应用类型： web、cmd

  - 设置环境变量
```
$ set App_ENV_application_appType=web
$ set APP_ENV_application_env=test
```
### 通过设置配置类型的环境变量，可覆盖yml配置文件的指定的配置项
- 变量名格式: 前缀APP_CONF_ + yml配置文件配置项路径(用_连接，大小写保持跟yml配置一致)
- 如：APP_CONF_application_recover_debugMode 映射 application.recover.debugMode 配置项
```
  APP_CONF_application_recover_debugMode环境变量的值将覆盖yml配置文件中的 application.recover.debugMode 配置项
```
 