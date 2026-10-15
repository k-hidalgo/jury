---
sidebar_position: 6
title: Groups
description: How to judge if you have multiple judging areas
---

# Judging with Multiple Groups

This is arguably the most complex feature in Jury, and it should only be used if you have a larger hackathon where projects are split up among rooms that are relatively far apart (ie. it would take a judge more than like 30 seconds to walk between rooms). If this applies to you, we have added a feature that allows for judges to stay in a room or area for a couple of projects before having to change rooms. This decreases their walking time while still maintaining fairness in the judging algorithm.

## Setup

To set up multi-group judging, toggle it in the settings:

![Multi-group judging settings](./assets/group-settings.png)

:::warning
Same as with tracks, toggling multi-group judging on/off during judging is a BAD idea (ie. don't do it!!)
:::

### Number and Size of Groups

The first option is to set the **number of groups**. This determines how many groups projects will be split into. The next option is a comma-separated list of the **sizes of each group**. Note that projects will be assigned to groups starting from the first, so make sure you do not create too many groups! Changing the sizes or number of groups will automatically update the group numbers of projects, so feel free to keep changing this to match your room setup during judging setup. This is again probably not a good idea during judging. 

### Switching Modes

There are 2 methods of switching judges between groups. We want to switch judges occasionally otherwise they would see an uneven number of projects from one group. 

1. The first and most common method is `auto`. This is used if you want to automatically switch between rooms after a **proportion** of projects from a room has been seen (eg. 10% of 30 projects in a room means a judge sees 3 projects then moves to the next room). This is a proportion not a single value due to rooms having different number of projects.
2. The more uncommon method is `manual`. A button will show up on the dashboard actions that will allow you to manually switch groups. One use case of manually switching is during [HackUTD 2024](https://ripple.hackutd.co), where we had two buildings for judging. We didn't want judges going between the different buildings too much, so we made 2 big groups. Each building consisted of a group, and halfway through judging we told everyone to switch buildings. This ensures that both buildings' projects are evenly compared while preventing judges from having to walk too much.

### Group Names

Although mostly cosmetic, the group names are displayed in the project expo page. The group names can be seen when you print out the expo page and help hackers easily identify where they need to go. Make sure this list is the same length as the number of groups.

## Assigning Judges to Groups

Judge groups are assigned **automatically** as judges are added through QR registration, the admin form, or CSV import. General judging and each track have separate staffing pools. By default, assignment balances judges evenly. Enabling **Assign Judges Proportionally to Group Size** weights general judging by all active projects and each track by only its eligible active projects in each group.

Track assignment first places a judge in every group with eligible projects, when enough judges are available. Additional judges are allocated proportionally. For example, a track with nine projects in one room and two in another receives one judge per room when two judges register. With eleven judges, it receives nine and two. Groups without eligible track projects receive no starting track judges.

Before projects are imported, track assignment is provisional: it uses configured group sizes in proportional mode and equal weights otherwise. After import, or after changing this setting, use **Reassign Judge Groups** before judging begins to update existing general and track accounts independently. Registration does not move previously assigned judges. Manual group switching and project selection are unchanged by this allocation setting. If for any reason the number of judges in a group is uneven, you can either manually move a single judge using the **Move Group** action in the admin table:

![Move groups](./assets/move-groups.png)

Or, you can click the "reassign groups" to reassign ALL judge groups. This *can* be done during judging as it will just shuffle around judges a bit to new groups; however, try to avoid this as much as possible! It is also not advised to reassign groups mid-judging if using the manual switching method as it may mess up the judging ratios!

![Reassign groups](./assets/reassign-groups.png)


## Overall Finalist Review

Under Settings → Judging Parameters, **Overall Finalist Review** is off by default.
Use the existing **Prioritize** action in the Projects table to mark a shortlist,
then enable review after checking initial coverage. Enabling requires confirmation.
The default target is **4 total completed overall views**, including earlier visits.
Targets must be positive whole numbers and can be changed during review.

For each overall judge, Jury keeps the existing active/seen/flag/busy/group rules.
Within those eligible projects it first covers projects below **Set Minimum Project
Views**, favoring those with the fewest completed overall views, then prioritizes
shortlisted projects below the finalist target. For example, a general minimum of
2 gives projects with one view coverage priority over finalists already seen twice.
Setting the general minimum to 0 disables this coverage step.
When there are no available shortlisted projects below target, normal assignment
continues. Enabling review with no prioritized projects leaves assignment unchanged.
Turning review off restores the original selection path, including legacy priority.

This is not an automatic finalist selector or a hard view cap. Projects remain
eligible for normal assignment after the target, and visits already underway can
finish. Organizers must choose the shortlist and monitor the room-level summary.
Minimum-view coverage protection is per judge's room, not an event-wide gate. Existing
group fallback still applies when a room has no eligible projects; review does not
move judges between rooms merely because another room has finalists.

The coverage panel counts active projects with zero completed overall views and
shortlisted projects below target, both overall and by room. Track visits do not
count toward these totals. The Unseen count reports zero-view projects only, not
all projects below the general minimum. Counts refresh every ten seconds or manually.

Track assignment, track view limits, scoring, rankings, existing assignments, and
halfway room switching are unchanged. Review does not start on a timer: organizers
enable it manually. New settings read safely on older databases without a migration.
