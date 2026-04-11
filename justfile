# Justfile for attn-tool

build:
    go build -o attn-tool ./cmd/attn

# Install symlinks to ~/.local/bin
install:
    mkdir -p ~/.local/bin
    ln -sf {{ justfile_directory() }}/attn-tool ~/.local/bin/attn
    ln -sf {{ justfile_directory() }}/attn-tool ~/.local/bin/tts

# Remove symlinks
uninstall:
    rm -f ~/.local/bin/attn ~/.local/bin/tts

# Speak text
# Usage: just speak "message"
speak text:
    ./attn-tool "{{ text }}"

# Speak with alert mode
alert text:
    ./attn-tool --alert "{{ text }}"

# Test groq (requires accepting terms: https://console.groq.com/playground?model=canopylabs%2Forpheus-v1-english)
test-groq:
    ./attn-tool -o /tmp/attn-test-groq.mp3 --provider groq "Testing groq TTS"

# Test minimax (requires API key with TTS access)
test-minimax:
    ./attn-tool -o /tmp/attn-test-minimax.mp3 --provider minimax "Testing minimax TTS"

# Verify build and help
test: build
    ./attn-tool --help
