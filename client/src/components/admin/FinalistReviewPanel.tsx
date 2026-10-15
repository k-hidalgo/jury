import { useCallback, useEffect, useRef, useState } from 'react';
import { getRequest, postRequest } from '../../api';
import TextInput from '../TextInput';
import Button from '../Button';

interface Coverage {
    enabled: boolean;
    target_views: number;
    unseen: number;
    shortlisted: number;
    below_target: number;
    groups: { group: number; name: string; unseen: number; shortlisted: number; below_target: number }[];
}

const FinalistReviewPanel = () => {
    const [coverage, setCoverage] = useState<Coverage | null>(null);
    const [target, setTarget] = useState(4);
    const [error, setError] = useState('');
    const [saving, setSaving] = useState(false);
    const loaded = useRef(false);
    const refresh = useCallback(async () => {
        const res = await getRequest<Coverage>('/admin/finalist-review', 'admin');
        if (res.status !== 200 || !res.data) {
            setError('Could not refresh finalist coverage. Refresh before relying on these counts.');
            return;
        }
        setCoverage(res.data);
        if (!loaded.current) {
            setTarget(res.data.target_views);
            loaded.current = true;
        }
        setError('');
    }, []);

    useEffect(() => {
        refresh();
        const interval = window.setInterval(refresh, 10000);
        return () => window.clearInterval(interval);
    }, [refresh]);

    const save = async (data: { finalist_review?: boolean; finalist_target_views?: number }) => {
        if (saving) return;
        setSaving(true);
        const res = await postRequest<OkResponse>('/admin/options', 'admin', data);
        if (res.status !== 200 || res.data?.ok !== 1) {
            setError('Could not save finalist settings. ' + (res.error || 'Please retry.'));
        } else {
            await refresh();
        }
        setSaving(false);
    };

    const toggle = () => {
        if (!coverage) return;
        if (!coverage.enabled && !window.confirm(
            'Enable overall finalist review? Unseen projects in each judge’s room come first, then prioritized projects below the target. Track judging is unchanged.'
        )) return;
        save({ finalist_review: !coverage.enabled });
    };

    const saveTarget = () => {
        if (!Number.isSafeInteger(target) || target < 1) {
            setError('Finalist target views must be a positive whole number.');
            return;
        }
        save({ finalist_target_views: target });
    };

    return (
        <section className="my-5" aria-label="Overall finalist review">
            <h3 className="text-xl font-bold text-primary mb-2">Overall Finalist Review</h3>
            <p className="text-light mb-3">
                Use Prioritize in the Projects table to mark your shortlist. When enabled,
                overall judges first cover unseen projects in their room, then shortlisted
                projects below the target, favoring fewer views. With no available finalists
                below target, normal assignment continues. Track judging is unchanged.
            </p>
            <label className="flex items-center gap-2 my-3">
                <input type="checkbox" checked={coverage?.enabled ?? false}
                    disabled={!coverage || saving} onChange={toggle} />
                Enable Overall Finalist Review
            </label>
            <div className="flex flex-wrap items-end gap-3 mb-3">
                <TextInput label="Finalist target views" text={target} setText={setTarget} number />
                <Button type="primary" onClick={saveTarget} disabled={!coverage || saving}>Save Finalist Target</Button>
            </div>
            <p className="text-sm text-light mb-3">
                Default: 4 total completed overall views, including earlier visits. This is a
                priority target, not a maximum: visits already underway finish normally and
                projects remain eligible for normal assignment afterward. The room switch and
                current assignments are unaffected. Turn review off to restore normal behavior.
            </p>
            {error && <p role="alert" className="text-error mb-2">{error}</p>}
            {coverage ? <>
                <p className="font-bold mb-2">
                    Overall projects unseen: {coverage.unseen} · Shortlisted: {coverage.shortlisted}
                    {' '}· Shortlisted below target ({coverage.target_views}): {coverage.below_target}
                </p>
                <div className="overflow-x-auto">
                    <table className="w-full text-left mb-2">
                        <thead><tr><th>Room</th><th>Unseen</th><th>Shortlisted</th><th>Below target</th></tr></thead>
                        <tbody>{coverage.groups.map((group) => <tr key={group.group}>
                            <td>{group.name}</td><td>{group.unseen}</td>
                            <td>{group.shortlisted}</td><td>{group.below_target}</td>
                        </tr>)}</tbody>
                    </table>
                </div>
                <p className="text-sm text-light">Active projects only; counts refresh every 10 seconds.
                    Coverage protection is per judge’s room, not an event-wide gate. Resolve unseen projects in both rooms.</p>
            </> : <p>Loading finalist coverage…</p>}
            <Button type="text" onClick={refresh}>Refresh Finalist Coverage</Button>
        </section>
    );
};

export default FinalistReviewPanel;
