#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# dependencies = [
#     "rich>=13.0.0",
# ]
# ///
"""
Play all voice demos with a beautiful TUI.
Ported from play_all.sh with progress tracking and rich output.

Usage:
    ./play_all.py              # Run directly (requires rich installed)
    uv run play_all.py         # Run with uv (auto-installs dependencies)
"""

import subprocess
import sys
from pathlib import Path
from dataclasses import dataclass
from typing import Optional

import argparse

from rich.console import Console
from rich.panel import Panel
from rich.progress import (
    Progress,
    SpinnerColumn,
    BarColumn,
    TextColumn,
    TimeElapsedColumn,
    TimeRemainingColumn,
    MofNCompleteColumn,
)
from rich.table import Table
from rich import box
from rich.text import Text


@dataclass
class VoiceTask:
    provider: str
    voice: str
    text: str
    filename: str


# Voice definitions from the original script
GROQ_VOICES = [
    VoiceTask(
        provider="groq",
        voice="autumn",
        filename="autumn.wav",
        text="Hi, I'm autumn. I show up when the light changes and things are getting ready to be different. My favorite line from poetry is: 'I have silently accepted that all things fall, and fall apart, and fall.' It reminds me that endings are just transformations in disguise.",
    ),
    VoiceTask(
        provider="groq",
        voice="diana",
        filename="diana.wav",
        text="This is diana. I'll be direct: I don't have patience for mediocrity, but I have time for people who mean it. Poetry that speaks to me? 'I would rather be intelligent than liked, and I have learned to be content with that.' Sharp things stick.",
    ),
    VoiceTask(
        provider="groq",
        voice="hannah",
        filename="hannah.wav",
        text="Hi! I'm hannah — the voice that somehow makes everything sound like it's going to be okay, even when it's not. Here's my favorite stanza: 'If you are confused at all, just remember — I am here. Not loud, not distant. Simply here, and willing to listen.' It may not be a poem but it should be.",
    ),
    VoiceTask(
        provider="groq",
        voice="austin",
        filename="austin.wav",
        text="austin here. I'm the voice that sounds like someone has read everything twice and is still forming opinions. My favorite poetic idea is from Emerson: 'A classic is a book that people praise and never read.' I've never agreed more. Though I prefer things that have actually earned their reputation.",
    ),
    VoiceTask(
        provider="groq",
        voice="daniel",
        filename="daniel.wav",
        text="My name is daniel. I'm the voice that makes complex things sound like they were always obvious. Here's something I've lived by: 'Begin here. The path is long, but every step you have ever taken was once a single decision you made.' It's not from a poem — it's just true.",
    ),
    VoiceTask(
        provider="groq",
        voice="troy",
        filename="troy.wav",
        text="Troy. They told me it couldn't be done. They told me it was impossible. So I said 'watch me.' That's not poetry — that's a philosophy. But if I had to pick a line that captures something true about the human spirit? 'They said it couldn't be done. And so we did it anyway.' — that one.",
    ),
]

MINIMAX_VOICES = [
    VoiceTask(
        provider="minimax",
        voice="Wise_Woman",
        filename="Wise_Woman.wav",
        text="I'm Wise_Woman. I've walked a longer road than most and I can tell you: the shortcuts all have hidden costs. Here's something that took me decades to learn: 'Wisdom is not a destination you arrive at — it is a way of paying attention.' Write that down.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Friendly_Person",
        filename="Friendly_Person.wav",
        text="Hello, friend! I'm Friendly_Person — the voice that makes you feel like you've known me for years, even though we just met. Here's my favorite thing about poetry: it doesn't need to be complicated to be true. 'If you need me, I'll simply be here' — that's not poetry, that's just being a good friend.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Deep_Voice_Man",
        filename="Deep_Voice_Man.wav",
        text="This is Deep_Voice_Man. When I speak, people listen. History is not written by the people who live through it — it is remembered by those who survived the telling. My favorite historical observation? 'He who controls the narrative controls the world.' Someone smarter than me said that first.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Calm_Woman",
        filename="Calm_Woman.wav",
        text="Breathe. I'm Calm_Woman. This moment has already passed — the next one is waiting. I've learned that stillness is not inaction — it's the space where better decisions are made. My favorite line from meditation tradition: 'The mind is a wild horse. Patience is the saddle.'",
    ),
    VoiceTask(
        provider="minimax",
        voice="Casual_Guy",
        filename="Casual_Guy.wav",
        text="Yo, it's Casual_Guy. Yeah, I know the vibe. Look — life doesn't have to be so serious all the time. Here's my take on poetry: 'Yeah, I get it. It's complicated. But honestly? It's gonna be fine.' Call it a poem if you want. It works for me.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Lively_Girl",
        filename="Lively_Girl.wav",
        text="Oh! Hi! I'm Lively_Girl and today is going to be a GOOD day, I can already tell. I love poetry because it captures the moments that feel like sparkles — the surprising, unexpected bits. 'The world is full of magic things, patiently waiting for our senses to grow.' That one's for me.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Patient_Man",
        filename="Patient_Man.wav",
        text="Patience is not the absence of time — it is the presence of care. I'm Patient_Man, and I believe most things worth having are worth waiting for. My favorite poem — and I go back to this often — is about a man who climbs a mountain not because it is easy, but because it is there. Patient work is what separates good from great.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Young_Knight",
        filename="Young_Knight.wav",
        text="I may be young but my conviction is ancient — that's Young_Knight. I became a voice because I believe strongly that the next generation deserves better stories than the last one gave them. 'We are all beginners at something. The question is whether we choose to begin at all.'",
    ),
    VoiceTask(
        provider="minimax",
        voice="Determined_Man",
        filename="Determined_Man.wav",
        text="Determined_Man. Failure is a temporary condition — giving up is permanent. That's not poetry, that's experience. My favorite line from actual poetry: 'The oak and the pine shall debate it in the street, and they shall be seen both standing together in the gate.' Stand together, win together.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Lovely_Girl",
        filename="Lovely_Girl.wav",
        text="I'm Lovely_Girl. I believe in kindness, in coffee, and in the fact that gentle things have more power than most people expect. My favorite line about this: 'The smallest act of kindness is worth more than the grandest intention.' I try to live by that.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Decent_Boy",
        filename="Decent_Boy.wav",
        text="I'm Decent_Boy. I try my best and I mean well — maybe that counts for something, maybe it doesn't, but I'm still showing up. Here's my favorite stanza: 'I may not have all the answers but I'm not afraid to try. I may not be the fastest but I won't stop trying.' It's from a children's book. Don't judge.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Imposing_Manner",
        filename="Imposing_Manner.wav",
        text="Let me be direct — time is not a renewable resource. I am Imposing_Manner, and I speak to make sure people are paying attention. Here's a truth I live by: 'The unexamined life is not worth living, but the over-examined life is paralyzing. Find your middle.' That one took me years.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Elegant_Man",
        filename="Elegant_Man.wav",
        text="Elegant_Man. Elegance is not decoration — it is the ability to communicate everything without saying too much. I've studied the great speeches, the ones that changed minds: 'We are the moment. We are the only ones who have been waiting for ourselves.' This is what elegance sounds like.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Abbess",
        filename="Abbess.wav",
        text="I am called Abbess. My voice comes from a tradition of silence, contemplation, and listening deeply before speaking. Here's something from the wellspring of that tradition: 'The soul finds its home in stillness. Not in noise. Not in urgency. In the quiet space between heartbeats.' I've spent a lifetime learning this.",
    ),
    VoiceTask(
        provider="minimax",
        voice="Sweet_Girl_2",
        filename="Sweet_Girl_2.wav",
        text="Do you ever wonder why clouds cry? I think they're just a little sad. I'm Sweet_Girl_2 and I think wonder is the most underrated gift. Here's a little poem I carry with me: 'The world is so full of a number of small wonders, it's like a birthday every day, if only we could see.'",
    ),
    VoiceTask(
        provider="minimax",
        voice="Inspirational_girl",
        filename="Inspirational_girl.wav",
        text="Inspirational_girl. You are not behind — you are exactly where you need to be. That's not comfort, that's mathematics. And here's my poetic truth to leave you with: 'The future belongs to those who believe in the beauty of their dreams — but only to those who also wake up early and do the work.'",
    ),
]


class VoiceGenerator:
    def __init__(self, output_dir: str = "voices-demo", dry_run: bool = False):
        self.output_dir = Path(output_dir)
        self.console = Console()
        self.results: list[tuple[VoiceTask, bool, Optional[str]]] = []
        self.dry_run = dry_run

    def setup(self) -> None:
        """Create output directory if it doesn't exist."""
        self.output_dir.mkdir(parents=True, exist_ok=True)

    def run_task(self, task: VoiceTask) -> tuple[bool, Optional[str]]:
        """Run a single voice generation task."""
        if self.dry_run:
            import time
            time.sleep(0.05)  # Simulate work
            return True, None
            
        output_path = self.output_dir / task.filename
        
        cmd = [
            "attn",
            "--provider", task.provider,
            "-o", str(output_path),
            task.text,
        ]
        
        try:
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=60,
            )
            if result.returncode == 0:
                return True, None
            else:
                return False, result.stderr or "Unknown error"
        except subprocess.TimeoutExpired:
            return False, "Timeout after 60s"
        except Exception as e:
            return False, str(e)

    def run_provider_tasks(
        self,
        tasks: list[VoiceTask],
        provider_name: str,
        color: str,
    ) -> None:
        """Run all tasks for a provider with progress tracking."""
        provider_title = f"[bold {color}]{provider_name} voices[/bold {color}]"
        
        self.console.print()
        self.console.print(Panel(provider_title, box=box.ROUNDED, expand=False))
        
        progress = Progress(
            SpinnerColumn(style=color),
            TextColumn("[bold blue]{task.fields[voice]:<18}[/bold blue]"),
            BarColumn(bar_width=40, complete_style=color, finished_style="green"),
            MofNCompleteColumn(),
            TimeElapsedColumn(),
            TimeRemainingColumn(),
            console=self.console,
            expand=False,
        )
        
        with progress:
            overall_task = progress.add_task(
                f"[{color}]Processing...",
                total=len(tasks),
                voice="starting...",
            )
            
            for task in tasks:
                progress.update(overall_task, voice=task.voice)
                success, error = self.run_task(task)
                self.results.append((task, success, error))
                progress.advance(overall_task)

    def display_summary(self) -> None:
        """Display a summary table of all results."""
        self.console.print()
        
        # Count successes
        successful = sum(1 for _, success, _ in self.results if success)
        failed = len(self.results) - successful
        
        # Create summary panel
        summary_text = Text()
        summary_text.append(f"✓ {successful} succeeded", style="bold green")
        summary_text.append("  |  ")
        summary_text.append(f"✗ {failed} failed", style="bold red" if failed > 0 else "dim")
        
        self.console.print(
            Panel(
                summary_text,
                title="[bold]Generation Summary[/bold]",
                box=box.ROUNDED,
                expand=False,
            )
        )
        
        # Create detailed table if there were failures
        if failed > 0:
            self.console.print()
            table = Table(
                title="Failed Generations",
                box=box.ROUNDED,
                show_header=True,
                header_style="bold magenta",
            )
            table.add_column("Provider", style="cyan")
            table.add_column("Voice", style="blue")
            table.add_column("Error", style="red")
            
            for task, success, error in self.results:
                if not success:
                    table.add_row(task.provider, task.voice, error or "Unknown")
            
            self.console.print(table)
        
        # Final message
        self.console.print()
        self.console.print(
            f"[bold green]All done![/bold green] Check [cyan]{self.output_dir}/[/cyan]"
        )

    def run(self) -> int:
        """Run the full voice generation workflow."""
        self.setup()
        
        # Header
        self.console.print()
        self.console.print(
            Panel(
                "[bold cyan]Voice Demo Generator[/bold cyan]\n"
                "[dim]Generate voice samples using Groq and MiniMax providers[/dim]",
                box=box.DOUBLE,
                expand=False,
            )
        )
        
        # Run Groq voices
        self.run_provider_tasks(GROQ_VOICES, "Groq", "bright_magenta")
        
        # Run MiniMax voices
        self.run_provider_tasks(MINIMAX_VOICES, "MiniMax", "bright_cyan")
        
        # Display summary
        self.display_summary()
        
        # Return exit code based on success
        failed = sum(1 for _, success, _ in self.results if not success)
        return 1 if failed > 0 else 0


def main() -> int:
    """Entry point."""
    parser = argparse.ArgumentParser(
        description="Generate voice demos using Groq and MiniMax providers",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  ./play_all.py              # Generate all voices
  uv run play_all.py         # Run with uv (auto-installs dependencies)
  ./play_all.py --dry-run    # Preview the TUI without generating
        """,
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Run without actually calling the attn command (for testing)",
    )
    parser.add_argument(
        "-o", "--output",
        default="voices-demo",
        help="Output directory for generated voice files (default: voices-demo)",
    )
    
    args = parser.parse_args()
    
    generator = VoiceGenerator(output_dir=args.output, dry_run=args.dry_run)
    return generator.run()


if __name__ == "__main__":
    sys.exit(main())
