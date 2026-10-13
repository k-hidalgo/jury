import ReactMarkdown, { Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { twMerge } from 'tailwind-merge';

interface MarkdownProps {
    /** Markdown text to display */
    text: string;

    /** Style with className */
    className?: string;
}

/**
 * Older project imports store newlines as a literal "\n" (backslash + n).
 * Convert those to real newlines, but only if the text has no real newlines,
 * so that code samples like printf("\n") in real Markdown are left alone.
 */
export const normalizeNewlines = (text: string) => {
    if (text.includes('\n')) return text;
    return text.replace(/\\n/g, '\n');
};

// Tailwind styles for each Markdown element, since Tailwind resets the browser defaults
const components: Components = {
    h1: ({ node, ...props }) => <h3 className="text-2xl font-bold mt-4 mb-2" {...props} />,
    h2: ({ node, ...props }) => <h4 className="text-xl font-bold mt-4 mb-2" {...props} />,
    h3: ({ node, ...props }) => <h5 className="text-lg font-bold mt-3 mb-1" {...props} />,
    h4: ({ node, ...props }) => <h6 className="font-bold mt-3 mb-1" {...props} />,
    h5: ({ node, ...props }) => <h6 className="font-bold mt-3 mb-1" {...props} />,
    h6: ({ node, ...props }) => <h6 className="font-bold mt-3 mb-1" {...props} />,
    p: ({ node, ...props }) => <p className="mb-3" {...props} />,
    a: ({ node, ...props }) => (
        <a
            className="text-primary underline break-words"
            target="_blank"
            rel="noopener noreferrer"
            {...props}
        />
    ),
    ul: ({ node, ...props }) => <ul className="list-disc pl-6 mb-3" {...props} />,
    ol: ({ node, ...props }) => <ol className="list-decimal pl-6 mb-3" {...props} />,
    li: ({ node, ...props }) => <li className="mb-1" {...props} />,
    blockquote: ({ node, ...props }) => (
        <blockquote className="border-l-4 border-lightest pl-3 mb-3 italic" {...props} />
    ),
    pre: ({ node, ...props }) => (
        <pre
            className="bg-background rounded-md p-3 mb-3 overflow-x-auto text-sm font-mono"
            {...props}
        />
    ),
    code: ({ node, className, ...props }) => (
        <code className={twMerge('font-mono text-sm', className)} {...props} />
    ),
    img: ({ node, ...props }) => <img className="max-w-full h-auto my-2" {...props} />,
    hr: ({ node, ...props }) => <hr className="border-lightest my-4" {...props} />,
    table: ({ node, ...props }) => (
        <div className="overflow-x-auto mb-3">
            <table className="border-collapse" {...props} />
        </div>
    ),
    th: ({ node, ...props }) => (
        <th className="border border-lightest px-2 py-1 font-bold text-left" {...props} />
    ),
    td: ({ node, ...props }) => <td className="border border-lightest px-2 py-1" {...props} />,
};

/**
 * Renders Markdown text, such as project descriptions imported from Devpost.
 * Raw HTML in the text is dropped.
 */
const Markdown = (props: MarkdownProps) => {
    if (props.text.trim() === '') return <p className={props.className}>[No Description]</p>;

    return (
        <div className={twMerge('break-words overflow-x-hidden', props.className)}>
            <ReactMarkdown remarkPlugins={[remarkGfm]} components={components} skipHtml>
                {normalizeNewlines(props.text)}
            </ReactMarkdown>
        </div>
    );
};

export default Markdown;
