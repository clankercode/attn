#!/bin/bash
set -e
mkdir -p voices-demo

echo "=== Groq voices ==="
attn --provider groq -o voices-demo/autumn.wav "autumn"
attn --provider groq -o voices-demo/diana.wav "diana"
attn --provider groq -o voices-demo/hannah.wav "hannah"
attn --provider groq -o voices-demo/austin.wav "austin"
attn --provider groq -o voices-demo/daniel.wav "daniel"
attn --provider groq -o voices-demo/troy.wav "troy"

echo ""
echo "=== MiniMax voices ==="
attn --provider minimax -o voices-demo/Wise_Woman.wav "Wise Woman"
attn --provider minimax -o voices-demo/Friendly_Person.wav "Friendly Person"
attn --provider minimax -o voices-demo/Deep_Voice_Man.wav "Deep Voice Man"
attn --provider minimax -o voices-demo/Calm_Woman.wav "Calm Woman"
attn --provider minimax -o voices-demo/Casual_Guy.wav "Casual Guy"
attn --provider minimax -o voices-demo/Lively_Girl.wav "Lively Girl"
attn --provider minimax -o voices-demo/Patient_Man.wav "Patient Man"
attn --provider minimax -o voices-demo/Young_Knight.wav "Young Knight"
attn --provider minimax -o voices-demo/Determined_Man.wav "Determined Man"
attn --provider minimax -o voices-demo/Lovely_Girl.wav "Lovely Girl"
attn --provider minimax -o voices-demo/Decent_Boy.wav "Decent Boy"
attn --provider minimax -o voices-demo/Imposing_Manner.wav "Imposing Manner"
attn --provider minimax -o voices-demo/Elegant_Man.wav "Elegant Man"
attn --provider minimax -o voices-demo/Abbess.wav "Abbess"
attn --provider minimax -o voices-demo/Sweet_Girl_2.wav "Sweet Girl"
attn --provider minimax -o voices-demo/Inspirational_girl.wav "Inspirational Girl"

echo ""
echo "All done! Check voices-demo/"