# CodeGraph Q&A：未使用代碼盤點

> 盤點日期：2026-07-10  
> 範圍：當前倉庫 206 個已索引文件、2,873 個符號、9,034 條邊。  
> 方法：CodeGraph MCP/CLI 的 callers、文件依賴與 SQLite 符號入邊交叉檢查；排除生成碼、測試入口、`main`/`init`、路由註冊、Wire/DI、介面實作與明顯的框架公開 API。

## 結論摘要

以下「高置信」項目在當前倉庫沒有可達入口或實際調用者；其餘只有「倉庫內零調用」但可能供外部使用的導出 API，另列於後文，不應直接刪除。

## 高置信：整個組件或子圖不可達

### 1. App Hook Provider 子圖

`example_main/main.go` 沒有收集這組 provider/manager；兩個構造器均無調用者，因此整個子圖不可達：

- `example_application/providers/apphook/app_fiber_hook.go:9` — `RegisterFiberAppCoreHook`
- `example_application/providers/apphook/app_fiber_hook_provider.go:9` — `FiberAppHookProvider`
- `example_application/providers/apphook/app_fiber_hook_provider.go:13` — `NewFiberAppHookProvider`
- `example_application/providers/apphook/app_hook_manager.go:8` — `AppCoreHookPManager`
- `example_application/providers/apphook/app_hook_manager.go:12` — `NewAppCoreHookPManager`

若這是預留的可選示例，應在文檔中標為 optional；否則可整包移除。
【批注】：1. 已在 mian.go 将该 app hook 的 provider/manager 添加到集合统一处理；2. example_xxx 主要为示例参考。

### 2. MongoDB 命令示例子圖

`MongodbService` 文件沒有任何其他索引文件依賴；`MongodbModel` 只被該不可達 service 使用：

- `example_application/module/command-module/service/mongodb_service.go:9` — `MongodbService`
- `example_application/module/command-module/service/mongodb_service.go:14` — `NewMongodbService`
- `example_application/module/command-module/service/mongodb_service.go:21` — `MongodbService.Test`
- `example_application/module/command-module/model/mongodb_model.go:11` — `MongodbModel`
- `example_application/module/command-module/model/mongodb_model.go:16` — `NewMongodbModel`
- `example_application/module/command-module/model/mongodb_model.go:24` — `MongodbModel.Test`

【批注】：1. example_xxx 为示例参考； 2. 该 MongodbService 服务仅用于命令行模式的测试，存在未完善调用的问题，不影响框架本身，暂忽略，待补充。

### 3. 未接入的 Cron 包裝器

- `example_application/command/component/cron.go:5` — `CronWrap`
- `example_application/command/component/cron.go:9` — `NewCronWrap`

該文件沒有任何倉庫內依賴或調用者。
【批注】：1. 示例参考，不影响框架本身；2. 暂无需处理。

### 4. 重複且未接入的 Validate 示例

`component/validate/example/` 沒有生產代碼依賴，而且內容與已實際接入的 `example_application/providers/validatecustom/` 重複：

- `component/validate/example/register_custom_tags_example.go:6` — `GetValidatorTagFuncs`
- `component/validate/example/tag_hascourses_example.go:11` — `HascoursesRegisterValidation`
- `component/validate/example/tag_hascourses_example.go:31` — `HascoursesRegisterTranslation`

【批注】：跟 example_xxx 重复可以无需处理。

### 5. 只被 benchmark 使用的替代實作

`component/jsonconvert/convert_opt.go` 的 `DataWrapOpt` 及其整套函數沒有生產調用者，只被 `convert_vs_opt_bench_test.go` 使用。它不是運行時代碼，但若仍需保留效能比較 benchmark，則不應刪除。
【批注】：仅用于测试，无需处理。

## 高置信：未使用的聲明、方法和類型

### 私有聲明

- `bootstrap/bootstrap.go:29` — `envAppTypeWeb`，只被同樣未使用的 `defaultAppConfigFile` 引用
- `bootstrap/bootstrap.go:30` — `envAppTypeCmd`
- `bootstrap/bootstrap.go:31` — `defaultAppConfigFile`
- `recover_recoveries_impl.go:27` — `debugFlag`
- `recover_recoveries_impl.go:28` — `debugFlagValue`
- `component/bufferpool/buffer.go:109` — `createStringBuilderPool`（註釋標明為示例，但沒有調用者）

【批注】：1. bootstrap 中的未使用变量已移除；2. recover 中的 debugFlag/debugFlagValue 经检查已用在 recover_error_handler_impl.go，保留；3. bufferpool 中的 createStringBuilderPool 仅为示例，可保留。

### Example Mongo 方法

- `example_application/module/example-module/repository/health_repository.go:30` — `HealthRepository.Test`
- `example_application/module/example-module/model/example_model.go:103` — `ExampleModel.SaveMany`
- `example_application/module/example-module/model/example_model.go:116` — `ExampleModel.UpdateExample`
- `example_application/module/example-module/model/example_model.go:138` — `ExampleModel.DeleteExample`

【批注】：example_xxx 示例参考，保留不变或删除。

### MySQL Service 中沒有入口的包裝方法

CLI 的 `test-orm` 只調用 `AutoMigrate`、`TestOk`、`TestOrm`。以下 service 方法沒有任何調用者：

- `CreateUser`（`:164`）
- `GetUserByID`（`:181`）
- `GetUsersByName`（`:198`）
- `ListUsers`（`:215`）
- `UpdateUser`（`:232`）
- `UpdateUserStruct`（`:249`）
- `DeleteUser`（`:266`）
- `HardDeleteUser`（`:283`）
- `BatchCreateUsers`（`:302`）
- `BatchDeleteUsers`（`:323`）
- `CreateUserWithClasses`（`:346`）

文件：`example_application/module/command-module/service/example_mysql_service.go`。

【批注】：example_xxx 示例参考，保留不变或删除。

### 只被上述死包裝方法使用、或完全未使用的 MySQL Model 方法

下列方法不在 `TestOrm` 的可達路徑上：

- `GetUsersByName`（`:101`）
- `ListUsers`（`:114`）
- `HardDeleteUser`（`:209`）
- `CreateClass`（`:229`）
- `GetClassByID`（`:241`）
- `GetClassesByUserID`（`:259`）
- `ListClasses`（`:272`）
- `UpdateClass`（`:308`）
- `DeleteClass`（`:330`）
- `BatchCreateUsers`（`:350`）
- `BatchDeleteUsers`（`:373`）
- `CreateUserWithClasses`（`:393`）

文件：`example_application/module/command-module/model/mysql_model.go`。

`CreateUser`、`GetUserByID`、`UpdateUser`、`UpdateUserStruct`、`DeleteUser` 仍被 `TestOrm` 直接調用，不屬於死代碼。

### 其他未使用類型/函數

- `example_application/apivo/example/responsevo/example_respvo.go:23` — `ExampleListRespVo`；`ExampleIdRespVo` 仍被 Fiber/Gin 的 `CreateExample` 使用
- `example_application/utils/common.go:12` — `DownloadImg2File`
- `example_application/utils/common.go:44` — `PageNext`；實際使用的是功能相同的 `PageParams`
- `example_application/utils/common.go:68` — `Round`
【批注】：1. example_xxx 示例参考；2. 已增加注释和删除重复的 PageNext 函数。

## 空佔位文件

以下文件沒有任何符號或依賴，可刪除而不影響代碼；若是目錄佔位需求，建議改用 README 說明：

- `component/jsoncodec/gojson.go`
- `example_application/command/application/commands/test_other_command.go`
- `example_application/command/application/constants.go`
- `example_application/module/common-module/attrs/attr1.go`
- `example_application/module/common-module/vars/vars.go`
- `example_application/module/example-module/model/example_mysql_model.go`
- `plugin/loader.go`
- `plugin/registry.go`

【批注】：1. example_xxx 为示例参考，无需处理；2. component/jsoncodec/gojson.go 保留后续补充；3. plugin/* 已迁移到 plugins/ 保留待完善。

## 中置信：倉庫內沒有消費者，但可能是刻意的外部 API

這些項目在當前倉庫沒有調用者，但均為導出 API；對 library 倉庫而言不能僅憑零入邊判定可刪：

- `component/mongodecimal/mongo_decimal.go:17` — `MongoDecimal`，只存在介面實作斷言，尚未註冊到 BSON registry
- `component/validate/validate_wrapper.go:154` — `GetDefaultLang`
- `example_application/module/command-module/service/example_mysql_service.go:33` — `RegisterKeyExampleMysqlService`
- `boot.go` 的 `RunApplicationStarter`、`Default`、`WithAppId`/`WithAppName`/`WithVersion` 等 option builders
- `cache/cache_option.go` 的多個 fluent setters/getters、`StableBloomFilter` 的公開操作與統計方法
- `response` 的非池化構造器、MsgPack 解析函數
- `globalmanager` 的 `Rebuild`、`ReleaseAll`、`Clear`
- `task.go` 的 `TaskWorker.GetMux`、`TaskWorker.GetServer`
- `utils/common.go` 的若干通用工具函數

處理這一組前應先確認兼容性承諾、下游使用者或發佈版本的 API policy。

【批注】：1. MongoDecimal 已在根目录下 database/dbmongo/mongo.go 中注册；2. 其他有特殊需要或待改造中，暂不处理。

## 誤報排除記錄

CodeGraph 對函數值、結構體欄位和 DI/Wire 的部分引用可能沒有建立普通 `calls` 邊。本次已人工排除：

- `p1`：被 `cli.StringFlag.Destination` 使用
- `exceptions`：被 `MergeExceptions` 讀寫
- `shard`：是 `ShardedBloomFilter.shards` 的元素類型
- `HandleExampleCreateTask`：以函數值註冊到 task map
- validatecustom 的 tag 函數：以函數值由 `GetValidatorTagFuncs` 返回
- Wire provider-set、生成注入器、路由 handler、介面實作方法

因此，本報告的「高置信」結果以完整可達子圖與具體源碼上下文為準，而不是單純依賴零入邊 SQL。
