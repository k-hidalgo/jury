import { useEffect, useRef, useState } from 'react';
import { errorAlert, timeSince } from '../../../util';
import DeletePopup from './DeletePopup';
import EditJudgePopup from './EditJudgePopup';
import { postRequest, putRequest } from '../../../api';
import { useAdminStore, useAdminTableStore, useOptionsStore } from '../../../store';
import { twMerge } from 'tailwind-merge';
import ActionsDropdown from '../../ActionsDropdown';
import MoveGroupPopup from './MoveGroupPopup';
import GroupLabel from './GroupLabel';
import JudgeRanksPopup from './JudgeRanksPopup';
import { useNavigate } from 'react-router-dom';
import { invitationLabel } from '../add-judges/InvitationPanel';

interface JudgeRowProps {
    judge: Judge;
    idx: number;
}

const JudgeRow = ({ judge, idx }: JudgeRowProps) => {
    const [popup, setPopup] = useState(false);
    const [resending, setResending] = useState(false);
    const [editPopup, setEditPopup] = useState(false);
    const [movePopup, setMovePopup] = useState(false);
    const [deletePopup, setDeletePopup] = useState(false);
    const [ranksPopup, setRanksPopup] = useState(false);
    const ref = useRef<HTMLDivElement>(null);
    const fetchJudges = useAdminStore((state) => state.fetchJudges);
    const options = useOptionsStore((state) => state.options);
    const selectedTrack = useOptionsStore((state) => state.selectedTrack);
    const selected = useAdminTableStore((state) => state.selected);
    const setSelected = useAdminTableStore((state) => state.setSelected);
    const projects = useAdminStore((state) => state.projects);
    const navigate = useNavigate();

    useEffect(() => {
        function closeClick(event: MouseEvent) {
            if (ref && ref.current && !ref.current.contains(event.target as Node)) {
                setPopup(false);
            }
        }

        // Bind the event listener
        document.addEventListener('mousedown', closeClick);
        return () => {
            // Unbind the event listener on clean up
            document.removeEventListener('mousedown', closeClick);
        };
    }, [ref]);

    const handleCheckedChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const newSelected = selected.slice();
        newSelected[idx] = e.target.checked;
        setSelected(newSelected);
    };

    const hideJudge = async () => {
        const res = await putRequest<OkResponse>(`/judge/hide/${judge.id}`, 'admin', {
            hide: judge.active,
        });
        if (res.status !== 200) {
            errorAlert(res);
            return;
        }

        alert(`Judge ${judge.active ? 'hidden' : 'un-hidden'} successfully!`);
        fetchJudges();
    };

    const resendInvitation = async () => {
        if (resending || !window.confirm(`Resend the existing login code to ${judge.name} (${judge.email})?`)) return;
        setResending(true);
        try {
            const res = await postRequest<{ok: number; invitations: InvitationReport}>(`/judge/${judge.id}/invitation`, 'admin', null);
            if (res.status !== 200 || !res.data) { errorAlert(res); return; }
            const report = res.data.invitations;
            alert(report.accepted ? 'Invitation accepted by the email server. Inbox delivery may take longer.'
                : report.failed ? 'Invitation failed. The account and code are saved; check the email address and retry.'
                : 'An invitation is already being sent. Refresh status before retrying.');
        } finally {
            setResending(false);
            fetchJudges();
            window.dispatchEvent(new Event('jury-invitations-updated'));
        }
    };

    const loginAsJudge = () => {
        navigate(`/judge/login?code=${judge.code}`);
    }

    const idToProj = (id: string) => {
        if (!id || id === '') {
            return 'None';
        }

        const proj = projects.find((p) => p.id === id);
        return proj ? `${proj.name} [T${proj.location}]` : 'None';
    };

    return (
        <>
            <tr
                key={idx}
                className={twMerge(
                    'border-t-2 border-backgroundDark duration-150',
                    !judge.active && 'bg-backgroundDark',
                    selected && selected[idx] && 'bg-primary/20'
                )}
            >
                <td className="px-2">
                    <input
                        type="checkbox"
                        checked={selected && selected[idx]}
                        onChange={(e) => {
                            handleCheckedChange(e);
                        }}
                        className="cursor-pointer hover:text-primary duration-100"
                    ></input>
                </td>
                <td>{judge.name}
                    <div className="text-xs text-light">Invitation: {invitationLabel(judge.invitation_status)}</div>
                </td>
                <td className="text-center">{judge.code}</td>
                {options.multi_group && (
                    <td className="text-center">
                        <GroupLabel group={judge.group} names={options.group_names} />
                    </td>
                )}
                <td className="text-center">{judge.seen}</td>
                <td className="text-center">{idToProj(judge.current)}</td>
                <td className="text-center">{timeSince(judge.last_activity)}</td>
                <td className="text-right font-bold flex align-center justify-end">
                    <ActionsDropdown
                        open={popup}
                        setOpen={setPopup}
                        actions={['Scores', 'Edit', judge.active ? 'Hide' : 'Unhide', 'Move Group', 'Login as Judge', resending ? 'Sending Invitation…' : 'Resend Invitation', 'Delete']}
                        actionFunctions={[
                            setRanksPopup.bind(null, true),
                            setEditPopup.bind(null, true),
                            hideJudge,
                            setMovePopup.bind(null, true),
                            loginAsJudge,
                            resendInvitation,
                            setDeletePopup.bind(null, true),
                        ]}
                        redIndices={[6]}
                    />
                    <span
                        className="cursor-pointer px-1 hover:text-primary duration-150"
                        onClick={() => {
                            setPopup(!popup);
                        }}
                    >
                        ···
                    </span>
                </td>
            </tr>
            <MoveGroupPopup enabled={movePopup} setEnabled={setMovePopup} item={judge} />
            <DeletePopup enabled={deletePopup} setEnabled={setDeletePopup} element={judge} />
            <EditJudgePopup enabled={editPopup} setEnabled={setEditPopup} judge={judge} />
            <JudgeRanksPopup enabled={ranksPopup} setEnabled={setRanksPopup} judge={judge} />
        </>
    );
};

export default JudgeRow;
