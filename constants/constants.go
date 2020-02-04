// Package constants vends constants used in various components of pin service, e.g., env var names
package constants

const (
	// -------------- env vars --------------
	// common
	EnvVerbose = "PIN_VERBOSE"
	// stores
	EnvRedisHost                   = "REDIS_HOST"
	EnvRedisPort                   = "REDIS_PORT"
	EnvRedisPasswd                 = "REDIS_PASSWD"
	EnvRedisDB                     = "REDIS_DB"
	EnvPinStoreJunkFetcherPoolSize = "PIN_STORE_JUNK_FETCHER_POOL_SIZE"
	// server
	EnvAppHost                  = "PIN_HOST"
	EnvAppPort                  = "PIN_PORT"
	EnvReqBodySizeMaxByte       = "PIN_REQ_BODY_SIZE_MAX_BYTE"
	EnvPinTitleSizeMaxByte      = "PIN_TITLE_SIZE_MAX_BYTE"
	EnvPinNoteSizeMaxByte       = "PIN_NOTE_SIZE_MAX_BYTE"
	EnvPinAttachmentSizeMaxByte = "PIN_ATTACHMENT_SIZE_MAX_BYTE"
	EnvPinAttachmentCntMax      = "PIN_ATTACHMENT_COUNT_MAX"
	// deleter
	EnvPinDeleterLocalCacheSize   = "PIN_DELETER_LOCAL_CACHE_SIZE"
	EnvDeleterSweepFreq           = "PIN_DELETER_SWEEP_FREQ"
	EnvDeleterMaxSweepLoad        = "PIN_DELETER_MAX_SWEEP_LOAD"
	EnvDeleterExecutorPoolSize    = "PIN_DELETER_EXEC_POOL_SIZE"
	EnvDeleterWIPCacheEntryExpiry = "PIN_DELETER_WIP_CACHE_ENTRY_EXPIRY"

	// -------------- error messages --------------
	ErrMsgRequestBodyTooLarge = "request body too large"

	// -------------- error messages --------------
	LogFieldFuncName = "funcName"
)
