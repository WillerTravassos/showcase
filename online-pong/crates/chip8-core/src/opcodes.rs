//! Opcode decode and execution.
//!
//! Each opcode family is a private method on [`crate::Chip8`] dispatched from
//! [`crate::Chip8::decode_and_execute`].

use crate::{Chip8, DISPLAY_HEIGHT, DISPLAY_WIDTH, Error, Result};

impl Chip8 {
    // FN Visibility in Rust
    // pub:                         Public to everything (including external crates if the module is public)
    // pub(crate):                  Visible anywhere within the current crate
    // pub(super):                  Visible only to the parent module and its children
    // pub(self) / No modifier:     Visible only within the current module
    // pub(path/to/module):         Visible within a specific designated ancestor module path

    /// Pattern matches on nibbles of opcode, i.e., the bits of the opcode and executes instruction.
    pub(super) fn decode_and_execute(&mut self, opcode: u16) -> Result<()> {
        // Splits the 16-bit opcode into four individual 4-bit hexadecimal digits (nibbles).
        // Example: 0x2AF5 becomes (0x2, 0xA, 0xF, 0x5) for clear pattern matching
        let nibbles =
            ((opcode >> 12) as u8, (opcode >> 8 & 0xF) as u8, (opcode >> 4 & 0xF) as u8, (opcode & 0xF) as u8);

        match nibbles {
            (0x0, 0x0, 0xE, 0x0) => {
                self.op_clear();
                Ok(())
            }
            (0x0, 0x0, 0xE, 0xE) => self.op_return(),
            (0x1, _, _, _) => {
                self.op_jump(opcode);
                Ok(())
            }
            (0x2, _, _, _) => self.op_call_subroutine(opcode),
            (0x3, _, _, _) => {
                self.op_skip_if_eq_immediate(opcode);
                Ok(())
            }
            (0x4, _, _, _) => {
                self.op_skip_if_ne_immediate(opcode);
                Ok(())
            }
            (0x5, _, _, 0x0) => {
                self.op_skip_if_eq_register(opcode);
                Ok(())
            }
            (0x9, _, _, 0x0) => {
                self.op_skip_if_ne_register(opcode);
                Ok(())
            }
            _ => Err(Error::InvalidOpcode(opcode)),
        }
    }

    /// Clears the CHIP-8 emulator display. Opcode: 0x00E0.
    pub(super) fn op_clear(&mut self) {
        self.display = [false; DISPLAY_WIDTH * DISPLAY_HEIGHT];
    }

    /// Return from current instruction in the stack. Opcode: 0x00EE
    pub(super) fn op_return(&mut self) -> Result<()> {
        if self.sp == 0 {
            return Err(Error::StackUnderflow);
        }

        self.sp -= 1;
        self.pc = self.stack[self.sp as usize];

        Ok(())
    }

    /// Jumps to the given opcode. Opcode 1NNN, where self.pc jumps to address NNN.
    pub(super) fn op_jump(&mut self, opcode: u16) {
        self.pc = Self::nnn_from_opcode(opcode);
    }

    /// Pushes current self.pc onto the stack and jumps to the given opcode. Opcode 2NNN, where NNN
    /// is the next subroutine address.
    pub(super) fn op_call_subroutine(&mut self, opcode: u16) -> Result<()> {
        if usize::from(self.sp) >= self.stack.len() {
            return Err(Error::StackOverflow);
        }

        // pc already advanced by tick; return lands on the instruction after the call.
        self.stack[self.sp as usize] = self.pc;
        self.sp += 1;
        self.pc = Self::nnn_from_opcode(opcode);

        Ok(())
    }

    /// Skips next instruction if V[X] equals to the last NN of the opcode. Opcode 3XNN,
    /// where X is the index of a CHIP-8 8bit register, and NN is the expected value in that
    /// register.
    pub(super) fn op_skip_if_eq_immediate(&mut self, opcode: u16) {
        let register_index = Self::x_nibble_from_opcode(opcode);
        let expected = Self::nn_from_opcode(opcode);

        if self.v[register_index] == expected {
            self.increase_program_counter();
        }
    }

    /// Skips next instruction if V[X] not equals to the last NN of the opcode. Opcode 4XNN,
    /// where X is the index of a CHIP-8 8bit register, and NN is the expected value in that
    /// register.
    pub(super) fn op_skip_if_ne_immediate(&mut self, opcode: u16) {
        let register_index = Self::x_nibble_from_opcode(opcode);
        let expected = Self::nn_from_opcode(opcode);

        if self.v[register_index] != expected {
            self.increase_program_counter();
        }
    }

    /// Skips next instruction if V[X] equals V[Y]. Opcode 5XY0,
    /// where X is the index of a CHIP-8 8bit register, and Y is the index of another register.
    pub(super) fn op_skip_if_eq_register(&mut self, opcode: u16) {
        let x = Self::x_nibble_from_opcode(opcode);
        let y = Self::y_nibble_from_opcode(opcode);

        if self.v[x] == self.v[y] {
            self.increase_program_counter();
        }
    }

    /// Skips next instruction if V[X] not equals V[Y]. Opcode 9XY0,
    /// where X is the index of a CHIP-8 8bit register, and Y is the index of another register.
    pub(super) fn op_skip_if_ne_register(&mut self, opcode: u16) {
        let x = Self::x_nibble_from_opcode(opcode);
        let y = Self::y_nibble_from_opcode(opcode);

        if self.v[x] != self.v[y] {
            self.increase_program_counter();
        }
    }

    fn nn_from_opcode(opcode: u16) -> u8 {
        (opcode & 0x00FF) as u8
    }

    fn x_nibble_from_opcode(opcode: u16) -> usize {
        ((opcode >> 8) & 0x0F) as usize
    }

    fn y_nibble_from_opcode(opcode: u16) -> usize {
        ((opcode >> 4) & 0x0F) as usize
    }

    fn nnn_from_opcode(opcode: u16) -> u16 {
        opcode & 0x0FFF
    }
}

#[cfg(test)]
mod opcode_tests {
    use pretty_assertions::assert_eq;
    use rand::RngExt;

    use crate::{Chip8, DISPLAY_HEIGHT, DISPLAY_WIDTH, Error, START_ADDRESS};

    #[test]
    fn op_clear_suceeds() {
        let empty_display = [false; DISPLAY_WIDTH * DISPLAY_HEIGHT];
        let mut emulator = Chip8::default();

        (1..10).for_each(|_| {
            let x = rand::rng().random_range(0..=DISPLAY_WIDTH - 1);
            let y = rand::rng().random_range(0..=DISPLAY_HEIGHT - 1);

            emulator.display[y * DISPLAY_WIDTH + x] = true;
        });

        assert_ne!(emulator.display, empty_display);

        emulator.op_clear();

        assert_eq!(emulator.display, empty_display);
    }

    #[test]
    fn op_return_succeeds() {
        let return_address = START_ADDRESS + 4;
        let mut emulator = Chip8::default();
        emulator.stack[0] = return_address;
        emulator.sp = 1;

        assert_eq!(emulator.pc, START_ADDRESS);
        assert!(emulator.op_return().is_ok());
        assert_eq!(emulator.pc, return_address);
        assert_eq!(emulator.sp, 0);
    }

    #[test]
    fn op_return_stack_underflow_fails() {
        let mut emulator = Chip8::default();

        assert!(emulator.op_return().is_err());
    }

    #[test]
    fn op_jump_succeeds() {
        let opcode = 0x1021;
        let mut emulator = Chip8::default();

        emulator.op_jump(opcode);

        assert_eq!(emulator.pc, 0x021);
    }

    #[test]
    fn op_call_subroutine_push_pc_and_jump_succeeds() {
        let opcode = 0x1021;
        let mut emulator = Chip8::default();
        let original_pc = emulator.pc;

        assert!(emulator.op_call_subroutine(opcode).is_ok());
        assert_eq!(emulator.sp, 1);
        assert_eq!(emulator.stack[0], original_pc);
        assert_eq!(emulator.pc, 0x021);
    }

    #[test]
    fn op_call_subroutine_stack_overflow_fails() {
        let opcode = 0x1021;
        let mut emulator = Chip8::default();

        (0..emulator.stack.len()).for_each(|i| {
            emulator.stack[i] = 1;
            emulator.sp += 1;
        });

        let result = emulator.op_call_subroutine(opcode);
        assert_eq!(Error::StackOverflow, result.unwrap_err());
    }

    #[test]
    fn op_skip_if_eq_immediate_increments_on_match() {
        let mut emulator = Chip8::default();
        let opcode = 0x3001;

        emulator.v[0] = 1;
        emulator.op_skip_if_eq_immediate(opcode);

        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn op_skip_if_eq_immediate_does_not_increment_on_mismatch() {
        let mut emulator = Chip8::default();
        let opcode = 0x3301;

        emulator.v[3] = 2;
        emulator.op_skip_if_eq_immediate(opcode);

        assert_eq!(emulator.pc, START_ADDRESS);
    }

    #[test]
    fn op_skip_if_eq_immediate_x_at_max_nibble() {
        let mut emulator = Chip8::default();
        let opcode = 0x3FFF;

        emulator.v[0xF] = 0xFF;
        emulator.op_skip_if_eq_immediate(opcode);

        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn op_skip_if_ne_immediate_increments_on_mismatch() {
        let mut emulator = Chip8::default();
        let opcode = 0x4502;

        emulator.v[5] = 1;
        emulator.op_skip_if_ne_immediate(opcode);

        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn op_skip_if_ne_immediate_does_not_increment_on_match() {
        let mut emulator = Chip8::default();
        let opcode = 0x4701;

        emulator.v[7] = 1;
        emulator.op_skip_if_ne_immediate(opcode);

        assert_eq!(emulator.pc, START_ADDRESS);
    }

    #[test]
    fn op_skip_if_ne_immediate_x_at_max_nibble() {
        let mut emulator = Chip8::default();
        let opcode = 0x4FFF;

        emulator.v[0xF] = 0xFF;
        emulator.op_skip_if_ne_immediate(opcode);

        assert_eq!(emulator.pc, START_ADDRESS);
    }

    #[test]
    fn op_skip_if_eq_register_increments_on_match() {
        let mut emulator = Chip8::default();
        let opcode = 0x5230;

        emulator.v[2] = 0x42;
        emulator.v[3] = 0x42;
        emulator.op_skip_if_eq_register(opcode);

        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn op_skip_if_eq_register_does_not_increment_on_mismatch() {
        let mut emulator = Chip8::default();
        let opcode = 0x5230;

        emulator.v[2] = 0x42;
        emulator.v[3] = 0x43;
        emulator.op_skip_if_eq_register(opcode);

        assert_eq!(emulator.pc, START_ADDRESS);
    }

    #[test]
    fn op_skip_if_eq_register_x_and_y_at_max_nibble() {
        let mut emulator = Chip8::default();
        let opcode = 0x5FE0;

        emulator.v[0xF] = 0xAB;
        emulator.v[0xE] = 0xAB;
        emulator.op_skip_if_eq_register(opcode);

        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn op_skip_if_ne_register_increments_on_mismatch() {
        let mut emulator = Chip8::default();
        let opcode = 0x9230;

        emulator.v[2] = 0x42;
        emulator.v[3] = 0x43;
        emulator.op_skip_if_ne_register(opcode);

        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn op_skip_if_ne_register_does_not_increment_on_match() {
        let mut emulator = Chip8::default();
        let opcode = 0x9230;

        emulator.v[2] = 0x42;
        emulator.v[3] = 0x42;
        emulator.op_skip_if_ne_register(opcode);

        assert_eq!(emulator.pc, START_ADDRESS);
    }

    #[test]
    fn op_skip_if_ne_register_x_and_y_at_max_nibble() {
        let mut emulator = Chip8::default();
        let opcode = 0x9FE0;

        emulator.v[0xF] = 0x01;
        emulator.v[0xE] = 0x02;
        emulator.op_skip_if_ne_register(opcode);

        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }
}
