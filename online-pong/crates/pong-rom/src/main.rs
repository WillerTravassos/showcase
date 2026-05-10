//! Pong ROM crate.
//!
//! The actual game source lives in `src/pong.s8`. A `build.rs` script invokes
//! `chip8-asm` at compile time to produce the `.ch8` binary. This crate has no
//! runtime logic of its own.

fn main() {
    println!("Hello, world!");
}
