import { useCallback, useEffect, useState } from 'react';
import { getRequest, postRequest } from '../../../api';
import Button from '../../Button';
import Card from '../../Card';

export const invitationLabel = (status?: string) => ({
    pending: 'Pending', sending: 'Sending', accepted: 'Accepted by email server',
    failed: 'Failed', not_sent: 'Not sent',
}[status || ''] || 'Unknown — not tracked');

interface InvitationSummary {
    counts: Record<string, number>;
    retryable: number;
    items: {id: string; name: string; email: string; track: string; status: string}[];
}

const InvitationPanel = () => {
    const [summary, setSummary] = useState<InvitationSummary | null>(null);
    const [busy, setBusy] = useState(false);
    const [message, setMessage] = useState('');
    const [error, setError] = useState('');
    const refresh = useCallback(async () => {
        const res = await getRequest<InvitationSummary>('/admin/invitations', 'admin');
        if (res.status !== 200 || !res.data) {
            setError('Could not refresh invitations. Refresh before retrying or importing again.');
            return;
        }
        setSummary(res.data);
        setError('');
    }, []);

    useEffect(() => {
        refresh();
        const interval = window.setInterval(refresh, 10000);
        window.addEventListener('jury-invitations-updated', refresh);
        return () => {
            window.clearInterval(interval);
            window.removeEventListener('jury-invitations-updated', refresh);
        };
    }, [refresh]);

    const retry = async () => {
        if (busy || !summary?.retryable || !window.confirm('Retry failed and pending invitations for active judges? Existing accounts and login codes will be reused.')) return;
        setBusy(true);
        setMessage('');
        try {
            const res = await postRequest<{ok: number; invitations: InvitationReport}>('/admin/invitations/retry', 'admin', null);
            if (res.status !== 200 || !res.data) {
                setMessage('Retry did not finish. Accounts remain saved. Refresh status before retrying again.');
                return;
            }
            const result = res.data.invitations;
            setMessage(`${result.accepted} accepted by email server; ${result.failed} failed; ${result.skipped} already handled or being sent.`);
        } finally {
            setBusy(false);
            await refresh();
        }
    };

    return <Card>
        <h2 className="text-2xl text-primary">Invitation Status</h2>
        <p className="text-light my-2">Active judges only. Status is saved across refreshes. Accepted means the email server accepted the message, not that it reached the inbox.</p>
        {summary ? <>
            <p className="my-2">Pending: {summary.counts.pending} · Sending: {summary.counts.sending} · Accepted: {summary.counts.accepted} · Failed: {summary.counts.failed}</p>
            <p className="text-sm text-light">Not sent: {summary.counts.not_sent} · Unknown: {summary.counts.unknown}. These accounts are excluded from bulk retries; use a judge’s Resend Invitation menu action if needed.</p>
            {summary.items.length > 0 && <div className="overflow-x-auto max-h-64 my-3">
                <table className="w-full text-left">
                    <thead><tr><th>Name</th><th>Email</th><th>Track</th><th>Invitation</th></tr></thead>
                    <tbody>{summary.items.map(item => <tr key={item.id}>
                        <td>{item.name}</td><td>{item.email}</td><td>{item.track || 'Overall'}</td><td>{invitationLabel(item.status)}</td>
                    </tr>)}</tbody>
                </table>
            </div>}
            <div className="flex flex-wrap gap-3 my-3">
                <Button type="primary" onClick={retry} disabled={busy || !!error || !summary.retryable}>
                    {busy ? 'Retrying Invitations…' : `Retry Failed / Pending Invitations (${summary.retryable})`}
                </Button>
                <Button type="outline" onClick={refresh}>Refresh Invitation Status</Button>
            </div>
        </> : <p>Loading invitations…</p>}
        <p className="text-sm text-light">Interrupted sending attempts become retryable after two minutes. An uncertain retry may send another copy of the same valid code. Accepted invitations are excluded from bulk retries.</p>
        {message && <p role="status" className="my-2">{message}</p>}
        {error && <p role="alert" className="text-error my-2">{error}</p>}
    </Card>;
};

export default InvitationPanel;
