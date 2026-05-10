//! Desktop frontend for the CHIP-8 emulator.
//!
//! Drives a `winit` event loop, renders display frames via `pixels`, maps
//! keyboard input to CHIP-8 keypad events, and routes audio through `cpal`.
//! Communicates with `chip8-core` over channels; no emulator state crosses
//! thread boundaries via shared memory.

fn main() {
    println!("Hello, world!");
}
