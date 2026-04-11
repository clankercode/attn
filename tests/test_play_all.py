import unittest

from play_all import (
    PlaybackAction,
    PlaybackState,
    ReviewAvailability,
    apply_playback_action,
    get_playback_action_for_key,
    get_review_availability,
    mpv_command_for_state,
)


class PlaybackControlTests(unittest.TestCase):
    def test_key_mapping_matches_control_hints(self) -> None:
        self.assertEqual(get_playback_action_for_key(" "), PlaybackAction.TOGGLE_PAUSE)
        self.assertEqual(get_playback_action_for_key("-"), PlaybackAction.SLOWER)
        self.assertEqual(get_playback_action_for_key("_"), PlaybackAction.SLOWER)
        self.assertEqual(get_playback_action_for_key("+"), PlaybackAction.FASTER)
        self.assertEqual(get_playback_action_for_key("="), PlaybackAction.FASTER)
        self.assertEqual(get_playback_action_for_key("0"), PlaybackAction.RESET_SPEED)
        self.assertEqual(get_playback_action_for_key("q"), PlaybackAction.QUIT)
        self.assertEqual(get_playback_action_for_key("Q"), PlaybackAction.QUIT)
        self.assertIsNone(get_playback_action_for_key("x"))

    def test_state_transitions_toggle_pause_and_clamp_speed(self) -> None:
        state = PlaybackState()

        state = apply_playback_action(state, PlaybackAction.TOGGLE_PAUSE)
        self.assertTrue(state.paused)
        self.assertEqual(state.speed, 1.0)

        state = apply_playback_action(state, PlaybackAction.TOGGLE_PAUSE)
        self.assertFalse(state.paused)

        for _ in range(10):
            state = apply_playback_action(state, PlaybackAction.FASTER)
        self.assertEqual(state.speed, 2.0)

        for _ in range(10):
            state = apply_playback_action(state, PlaybackAction.SLOWER)
        self.assertEqual(state.speed, 0.5)

        state = apply_playback_action(state, PlaybackAction.RESET_SPEED)
        self.assertEqual(state.speed, 1.0)

    def test_quit_sets_single_source_of_truth_state(self) -> None:
        state = PlaybackState()

        state = apply_playback_action(state, PlaybackAction.QUIT)

        self.assertTrue(state.quit_requested)
        self.assertEqual(state.status_label, "Quitting")

    def test_mpv_command_generation_matches_state(self) -> None:
        playing_state = PlaybackState(paused=False, speed=1.25)
        paused_state = PlaybackState(paused=True, speed=0.75)

        self.assertEqual(
            mpv_command_for_state(playing_state, PlaybackAction.TOGGLE_PAUSE),
            {"command": ["set_property", "pause", True]},
        )
        self.assertEqual(
            mpv_command_for_state(paused_state, PlaybackAction.TOGGLE_PAUSE),
            {"command": ["set_property", "pause", False]},
        )
        self.assertEqual(
            mpv_command_for_state(playing_state, PlaybackAction.FASTER),
            {"command": ["set_property", "speed", 1.5]},
        )
        self.assertEqual(
            mpv_command_for_state(paused_state, PlaybackAction.RESET_SPEED),
            {"command": ["set_property", "speed", 1.0]},
        )
        self.assertIsNone(mpv_command_for_state(playing_state, PlaybackAction.QUIT))

    def test_review_availability_explains_skip_reasons(self) -> None:
        self.assertEqual(
            get_review_availability(
                has_input_tty=False,
                has_output_tty=True,
                has_playable_files=True,
                mpv_available=True,
            ),
            ReviewAvailability(False, "stdin is not a TTY"),
        )
        self.assertEqual(
            get_review_availability(
                has_input_tty=True,
                has_output_tty=False,
                has_playable_files=True,
                mpv_available=True,
            ),
            ReviewAvailability(False, "stdout is not a TTY"),
        )
        self.assertEqual(
            get_review_availability(
                has_input_tty=True,
                has_output_tty=True,
                has_playable_files=False,
                mpv_available=True,
            ),
            ReviewAvailability(False, "no playable files were generated"),
        )
        self.assertEqual(
            get_review_availability(
                has_input_tty=True,
                has_output_tty=True,
                has_playable_files=True,
                mpv_available=False,
            ),
            ReviewAvailability(False, "mpv is not installed"),
        )
        self.assertEqual(
            get_review_availability(
                has_input_tty=True,
                has_output_tty=True,
                has_playable_files=True,
                mpv_available=True,
            ),
            ReviewAvailability(True, None),
        )


if __name__ == "__main__":
    unittest.main()
