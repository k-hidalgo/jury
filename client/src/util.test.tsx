import { describe, expect, it } from 'vitest';
import { displayGroup, parseGroupInput } from './util';

describe('displayGroup', () => {
    it('shows stored groups starting at 1', () => {
        expect(displayGroup(0)).toBe(1);
        expect(displayGroup(2)).toBe(3);
    });
});

describe('parseGroupInput', () => {
    it('converts groups typed starting at 1 to stored groups starting at 0', () => {
        expect(parseGroupInput('1', 3)).toBe(0);
        expect(parseGroupInput('3', 3)).toBe(2);
        expect(parseGroupInput(' 2 ', 3)).toBe(1);
    });

    it('round trips with displayGroup', () => {
        for (let group = 0; group < 5; group++) {
            expect(parseGroupInput(String(displayGroup(group)), 5)).toBe(group);
        }
    });

    it('rejects groups out of range', () => {
        expect(parseGroupInput('0', 3)).toBeNull();
        expect(parseGroupInput('4', 3)).toBeNull();
        expect(parseGroupInput('1', 0)).toBeNull();
    });

    it('rejects input that is not a whole number', () => {
        expect(parseGroupInput('', 3)).toBeNull();
        expect(parseGroupInput('-1', 3)).toBeNull();
        expect(parseGroupInput('1.5', 3)).toBeNull();
        expect(parseGroupInput('two', 3)).toBeNull();
        expect(parseGroupInput('1e0', 3)).toBeNull();
    });
});
