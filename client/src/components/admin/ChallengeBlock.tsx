import { useEffect, useState } from 'react';
import { getRequest } from '../../api';
import { errorAlert } from '../../util';

interface ChallengeBlockProps {
    excludedTracks?: string[];
}

const ChallengeBlock = ({ excludedTracks = [] }: ChallengeBlockProps) => {
    const [challenges, setChallenges] = useState<string[]>([]);

    useEffect(() => {
        async function fetchData() {
            const res = await getRequest<string[]>('/challenges', '');

            if (res.status !== 200) {
                errorAlert(res);
                return;
            }

            setChallenges(res.data as string[]);
        }

        fetchData();
    }, []);

    const visibleChallenges = challenges
        .filter((name) => !excludedTracks.includes(name))
        .sort((a, b) =>
            a.localeCompare(b, 'en', { sensitivity: 'base' }) ||
            a.localeCompare(b, 'en')
        );

    return (
        <div className="border-2 border-primary bg-primary/10 rounded-md p-2 my-2">
            <h1 className="text-lg md:text-xl font-bold">Challenge List</h1>
            <p className="text-sm md:text-md text-light">
                {visibleChallenges.length > 0
                    ? visibleChallenges.join(', ')
                    : 'No remaining categories to list.'}
            </p>
        </div>
    );
};

export default ChallengeBlock;
