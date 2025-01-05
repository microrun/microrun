inherit go

# Go static compilation settings for pure Go binaries
CGO_ENABLED = "0"
GO_DYNLINK:aarch64 = ""
GO_EXTLDFLAGS:append = " -static"
GOBUILDFLAGS:remove = " -buildmode=pie"