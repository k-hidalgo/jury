import { useLayoutEffect, useRef } from 'react';

interface NameListInputProps {
    label: string;
    value: string;
    setValue: React.Dispatch<React.SetStateAction<string>>;
}

const NameListInput = ({ label, value, setValue }: NameListInputProps) => {
    const inputRef = useRef<HTMLTextAreaElement>(null);

    useLayoutEffect(() => {
        const input = inputRef.current;
        if (!input) return;

        input.style.height = 'auto';
        input.style.height = `${input.scrollHeight}px`;
    }, [value]);

    return (
        <div className="flex flex-col w-full my-2">
            <label htmlFor={label} className="text-sm text-light mb-[2px]">
                {label} (one per line)
            </label>
            <textarea
                ref={inputRef}
                id={label}
                value={value}
                rows={3}
                className="w-full p-2 text-md border-lightest border-2 rounded-sm focus:border-primary outline-none overflow-hidden resize-none"
                placeholder="One complete track name per line"
                onChange={(event) => setValue(event.target.value)}
            />
        </div>
    );
};

export default NameListInput;
