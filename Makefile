.PHONY: install build

install: build
	install -d $(DESTDIR)/usr/local/bin
	ln -sf $(PWD)/attn-tool $(DESTDIR)/usr/local/bin/attn
	ln -sf $(PWD)/attn-tool $(DESTDIR)/usr/local/bin/tts

build:
	go build -o attn-tool ./cmd/attn
