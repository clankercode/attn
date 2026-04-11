# Justfile for attn-tool

build:
    go build -o attn-tool ./cmd/attn

# Install symlinks to /usr/local/bin (requires sudo)
install:
    sudo install -d /usr/local/bin
    sudo ln -sf {{ justfile_directory() }}/attn-tool /usr/local/bin/attn
    sudo ln -sf {{ justfile_directory() }}/attn-tool /usr/local/bin/tts

# Remove symlinks
uninstall:
    sudo rm -f /usr/local/bin/attn /usr/local/bin/tts

# Speak text (keys loaded from ~/.config/attn/config.yaml)
speak text:
    ./attn-tool "{{ text }}"

# Speak with alert mode
alert text:
    ./attn-tool "{{ text }}" --alert

# Test groq (may require accepting terms at https://console.groq.com/playground?model=canopylabs%2Forpheus-v1-english)
test-groq:
    ./attn-tool "Testing groq TTS" --provider groq -o /tmp/attn-test-groq.mp3

# Test minimax
test-minimax:
    ./attn-tool "Testing minimax TTS" --provider minimax -o /tmp/attn-test-minimax.mp3

# Verify help output
test: build
    ./attn-tool --help
