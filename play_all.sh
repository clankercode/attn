#!/bin/bash
set -e
mkdir -p voices-demo

echo "=== Groq voices ==="
attn --provider groq -o voices-demo/autumn.wav "Hi, I'm autumn. I show up when the light changes and things are getting ready to be different. My favorite line from poetry is: 'I have silently accepted that all things fall, and fall apart, and fall.' It reminds me that endings are just transformations in disguise."

attn --provider groq -o voices-demo/diana.wav "This is diana. I'll be direct: I don't have patience for mediocrity, but I have time for people who mean it. Poetry that speaks to me? 'I would rather be intelligent than liked, and I have learned to be content with that.' Sharp things stick."

attn --provider groq -o voices-demo/hannah.wav "Hi! I'm hannah — the voice that somehow makes everything sound like it's going to be okay, even when it's not. Here's my favorite stanza: 'If you are confused at all, just remember — I am here. Not loud, not distant. Simply here, and willing to listen.' It may not be a poem but it should be."

attn --provider groq -o voices-demo/austin.wav "austin here. I'm the voice that sounds like someone has read everything twice and is still forming opinions. My favorite poetic idea is from Emerson: 'A classic is a book that people praise and never read.' I've never agreed more. Though I prefer things that have actually earned their reputation."

attn --provider groq -o voices-demo/daniel.wav "My name is daniel. I'm the voice that makes complex things sound like they were always obvious. Here's something I've lived by: 'Begin here. The path is long, but every step you have ever taken was once a single decision you made.' It's not from a poem — it's just true."

attn --provider groq -o voices-demo/troy.wav "Troy. They told me it couldn't be done. They told me it was impossible. So I said 'watch me.' That's not poetry — that's a philosophy. But if I had to pick a line that captures something true about the human spirit? 'They said it couldn't be done. And so we did it anyway.' — that one."

echo ""
echo "=== MiniMax voices ==="
attn --provider minimax -o voices-demo/Wise_Woman.wav "I'm Wise_Woman. I've walked a longer road than most and I can tell you: the shortcuts all have hidden costs. Here's something that took me decades to learn: 'Wisdom is not a destination you arrive at — it is a way of paying attention.' Write that down."

attn --provider minimax -o voices-demo/Friendly_Person.wav "Hello, friend! I'm Friendly_Person — the voice that makes you feel like you've known me for years, even though we just met. Here's my favorite thing about poetry: it doesn't need to be complicated to be true. 'If you need me, I'll simply be here' — that's not poetry, that's just being a good friend."

attn --provider minimax -o voices-demo/Deep_Voice_Man.wav "This is Deep_Voice_Man. When I speak, people listen. History is not written by the people who live through it — it is remembered by those who survived the telling. My favorite historical observation? 'He who controls the narrative controls the world.' Someone smarter than me said that first."

attn --provider minimax -o voices-demo/Calm_Woman.wav "Breathe. I'm Calm_Woman. This moment has already passed — the next one is waiting. I've learned that stillness is not inaction — it's the space where better decisions are made. My favorite line from meditation tradition: 'The mind is a wild horse. Patience is the saddle.'"

attn --provider minimax -o voices-demo/Casual_Guy.wav "Yo, it's Casual_Guy. Yeah, I know the vibe. Look — life doesn't have to be so serious all the time. Here's my take on poetry: 'Yeah, I get it. It's complicated. But honestly? It's gonna be fine.' Call it a poem if you want. It works for me."

attn --provider minimax -o voices-demo/Lively_Girl.wav "Oh! Hi! I'm Lively_Girl and today is going to be a GOOD day, I can already tell. I love poetry because it captures the moments that feel like sparkles — the surprising, unexpected bits. 'The world is full of magic things, patiently waiting for our senses to grow.' That one's for me."

attn --provider minimax -o voices-demo/Patient_Man.wav "Patience is not the absence of time — it is the presence of care. I'm Patient_Man, and I believe most things worth having are worth waiting for. My favorite poem — and I go back to this often — is about a man who climbs a mountain not because it is easy, but because it is there. Patient work is what separates good from great."

attn --provider minimax -o voices-demo/Young_Knight.wav "I may be young but my conviction is ancient — that's Young_Knight. I became a voice because I believe strongly that the next generation deserves better stories than the last one gave them. 'We are all beginners at something. The question is whether we choose to begin at all.'"

attn --provider minimax -o voices-demo/Determined_Man.wav "Determined_Man. Failure is a temporary condition — giving up is permanent. That's not poetry, that's experience. My favorite line from actual poetry: 'The oak and the pine shall debate it in the street, and they shall be seen both standing together in the gate.' Stand together, win together."

attn --provider minimax -o voices-demo/Lovely_Girl.wav "I'm Lovely_Girl. I believe in kindness, in coffee, and in the fact that gentle things have more power than most people expect. My favorite line about this: 'The smallest act of kindness is worth more than the grandest intention.' I try to live by that."

attn --provider minimax -o voices-demo/Decent_Boy.wav "I'm Decent_Boy. I try my best and I mean well — maybe that counts for something, maybe it doesn't, but I'm still showing up. Here's my favorite stanza: 'I may not have all the answers but I'm not afraid to try. I may not be the fastest but I won't stop trying.' It's from a children's book. Don't judge."

attn --provider minimax -o voices-demo/Imposing_Manner.wav "Let me be direct — time is not a renewable resource. I am Imposing_Manner, and I speak to make sure people are paying attention. Here's a truth I live by: 'The unexamined life is not worth living, but the over-examined life is paralyzing. Find your middle.' That one took me years."

attn --provider minimax -o voices-demo/Elegant_Man.wav "Elegant_Man. Elegance is not decoration — it is the ability to communicate everything without saying too much. I've studied the great speeches, the ones that changed minds: 'We are the moment. We are the only ones who have been waiting for ourselves.' This is what elegance sounds like."

attn --provider minimax -o voices-demo/Abbess.wav "I am called Abbess. My voice comes from a tradition of silence, contemplation, and listening deeply before speaking. Here's something from the wellspring of that tradition: 'The soul finds its home in stillness. Not in noise. Not in urgency. In the quiet space between heartbeats.' I've spent a lifetime learning this."

attn --provider minimax -o voices-demo/Sweet_Girl_2.wav "Do you ever wonder why clouds cry? I think they're just a little sad. I'm Sweet_Girl_2 and I think wonder is the most underrated gift. Here's a little poem I carry with me: 'The world is so full of a number of small wonders, it's like a birthday every day, if only we could see.'"

attn --provider minimax -o voices-demo/Inspirational_girl.wav "Inspirational_girl. You are not behind — you are exactly where you need to be. That's not comfort, that's mathematics. And here's my poetic truth to leave you with: 'The future belongs to those who believe in the beauty of their dreams — but only to those who also wake up early and do the work.'"

echo ""
echo "All done! Check voices-demo/"
