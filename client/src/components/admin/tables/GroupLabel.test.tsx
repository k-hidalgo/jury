import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import GroupLabel from './GroupLabel';

const render = (group: number, names: string[]) =>
    renderToStaticMarkup(<GroupLabel group={group} names={names} />);

describe('GroupLabel', () => {
    it('shows the group number starting at 1 and its name', () => {
        const html = render(1, ['Main Hall', 'Room 101', 'Room 102']);
        expect(html).toContain('>2</span>');
        expect(html).toContain('>Room 101</span>');
        expect(html).not.toContain('Main Hall');
    });

    it('shows only the number if the group has no name', () => {
        const html = render(3, ['Main Hall', 'Room 101']);
        expect(html).toBe('<span class="font-bold">4</span>');
    });

    it('shows only the number if the name is blank', () => {
        const html = render(0, ['  ']);
        expect(html).toBe('<span class="font-bold">1</span>');
    });
});
