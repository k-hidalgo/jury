import { displayGroup } from '../../../util';

interface GroupLabelProps {
    /* Stored group (starting at 0) */
    group: number;

    /* Names of all groups */
    names: string[];
}

/**
 * Shows a group's number (starting at 1) along with its name, if it has one
 */
const GroupLabel = (props: GroupLabelProps) => {
    const name = props.names[props.group]?.trim();

    return (
        <>
            <span className="font-bold">{displayGroup(props.group)}</span>
            {/* Keep the name on one line, wrapping below the number if needed */}
            {name && (
                <>
                    {' '}
                    <span className="text-light whitespace-nowrap">{name}</span>
                </>
            )}
        </>
    );
};

export default GroupLabel;
