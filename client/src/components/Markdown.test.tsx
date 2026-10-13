import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import Markdown, { normalizeNewlines } from './Markdown';

const render = (text: string) => renderToStaticMarkup(<Markdown text={text} />);

describe('normalizeNewlines', () => {
    it('converts literal \\n to newlines when there are no real newlines', () => {
        expect(normalizeNewlines('First\\nSecond')).toBe('First\nSecond');
    });

    it('leaves literal \\n alone when the text has real newlines', () => {
        const text = '## Code\n\n`printf("\\n")`';
        expect(normalizeNewlines(text)).toBe(text);
    });
});

describe('Markdown', () => {
    it('shows a placeholder for empty descriptions', () => {
        expect(render('')).toContain('[No Description]');
        expect(render('   \n  ')).toContain('[No Description]');
    });

    it('renders a Devpost-style description', () => {
        const html = render(
            [
                '## Inspiration',
                'We wanted to make **AR** accessible to *everyone*.',
                '',
                '## What it does',
                '- Drag and drop scenes',
                '- Share with friends',
                '',
                '1. Open the app',
                '2. Build a scene',
                '',
                '## Built with',
                '`React`, `Go`, and ~~jQuery~~',
            ].join('\n')
        );

        expect(html).toContain('>Inspiration</h4>');
        expect(html).toContain('<strong>AR</strong>');
        expect(html).toContain('<em>everyone</em>');
        expect(html).toMatch(/<ul[^>]*>\s*<li[^>]*>Drag and drop scenes<\/li>/);
        expect(html).toMatch(/<ol[^>]*>\s*<li[^>]*>Open the app<\/li>/);
        expect(html).toContain('>React</code>');
        expect(html).toContain('<del>jQuery</del>');
    });

    it('renders old-style literal \\n descriptions as separate paragraphs', () => {
        const html = render('First paragraph.\\nSecond paragraph.\\n\\nThird paragraph.');
        expect(html).toContain('>First paragraph.');
        expect(html).toContain('Third paragraph.</p>');
        expect(html).not.toContain('\\n');
    });

    it('opens links in a new tab', () => {
        const html = render('See [our demo](https://example.com/demo) or https://example.com/repo');
        expect(html).toContain('href="https://example.com/demo"');
        expect(html).toContain('target="_blank"');
        expect(html).toContain('rel="noopener noreferrer"');

        // GFM autolinks plain URLs
        expect(html).toContain('href="https://example.com/repo"');
    });

    it('renders GFM tables', () => {
        const html = render('| Feature | Status |\n| --- | --- |\n| Login | Done |');
        expect(html).toMatch(/<table[^>]*>/);
        expect(html).toMatch(/<th[^>]*>Feature<\/th>/);
        expect(html).toMatch(/<td[^>]*>Done<\/td>/);
    });

    it('renders images', () => {
        const html = render('![Screenshot](https://example.com/screenshot.png)');
        expect(html).toMatch(/<img[^>]*src="https:\/\/example.com\/screenshot.png"/);
        expect(html).toContain('alt="Screenshot"');
    });

    it('drops raw HTML instead of rendering or showing it', () => {
        const html = render(
            'Hello\n\n<script>alert("xss")</script>\n\n<img src="x" onerror="alert(1)">\n\nSome <b>bold</b> text'
        );
        expect(html).not.toContain('xss');
        expect(html).not.toContain('onerror');
        expect(html).not.toContain('<b>');
        expect(html).not.toContain('&lt;');
        expect(html).toContain('Hello');
        expect(html).toContain('Some bold text');
    });

    it('removes javascript: links', () => {
        const html = render('[click me](javascript:alert(1))');
        expect(html).not.toContain('javascript:');
        expect(html).toContain('click me');
    });
});
