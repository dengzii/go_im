package groups

import (
	"errors"
	"go_im/im/api/apidep"
	"go_im/im/api/router"
	"go_im/im/dao"
	"go_im/im/dao/groupdao"
	"go_im/im/message"
)

type Interface interface {
}

type GroupApi struct {
}

func (m *GroupApi) CreateGroup(ctx *route.Context, request *CreateGroupRequest) error {

	g, cid, err := m.createGroup(request.Name, ctx.Uid)
	if err != nil {
		return err
	}
	//members, _ := groupdao.GroupDao.GetMembers(g.Gid)
	//c := user.ContactResponse{
	//	Friends: []*user.InfoResponse{},
	//	Groups: []*GroupResponse{{
	//		Group:   *g,
	//		Members: members,
	//	}},
	//}
	//respond(ctx.Uid, -1, "api.ActionUserAddFriend", c)

	// create user chat by default
	uc, err := dao.ChatDao.NewUserChat(cid, ctx.Uid, g.Gid, dao.ChatTypeGroup)
	if err != nil {
		return err
	}
	ctx.Response(message.NewMessage(-1, "", uc))

	// add invited member to group
	if len(request.Member) > 0 {
		nMsg := &route.Context{
			Uid: ctx.Uid,
			Seq: -1,
		}
		nReq := &AddMemberRequest{
			Gid: g.Gid,
			Uid: request.Member,
		}
		err = m.AddGroupMember(nMsg, nReq)
		if err != nil {
			ctx.Response(message.NewMessage(-1, "", "add member failed, "+err.Error()))
		}
	}
	ctx.Response(message.NewMessage(ctx.Seq, "", "create success"))
	return nil
}

func (m *GroupApi) GetGroupMember(ctx *route.Context, request *GetGroupMemberRequest) error {

	members, err := groupdao.GroupDao.GetMembers(request.Gid)
	if err != nil {
		return err
	}

	ms := make([]*GroupMemberResponse, 0, len(members))
	for _, member := range members {
		ms = append(ms, &GroupMemberResponse{
			Uid:        member.Uid,
			Nickname:   "",
			RemarkName: member.Remark,
			Type:       member.Flag,
			Online:     true,
		})
	}

	ctx.Response(message.NewMessage(ctx.Seq, "", "success"))
	return nil
}

func (m *GroupApi) GetGroupInfo(ctx *route.Context, request *GroupInfoRequest) error {

	var groups []*GroupResponse

	for _, gid := range request.Gid {
		group1, e := groupdao.GroupDao.GetGroup(gid)
		if e != nil {
			return e
		}
		ms, _ := groupdao.GroupDao.GetMembers(gid)
		gr := GroupResponse{
			Group:   *group1,
			Members: ms,
		}
		groups = append(groups, &gr)
	}
	ctx.Response(message.NewMessage(ctx.Seq, "", "create success"))
	return nil
}

func (m *GroupApi) RemoveMember(ctx *route.Context, request *RemoveMemberRequest) error {

	for _, uid := range request.Uid {
		err := groupdao.GroupDao.RemoveMember(request.Gid, uid)
		if err != nil {
			return err
		}
		_ = apidep.GroupManager.RemoveMember(request.Gid, uid)
		notifyResp := message.NewMessage(-1, "api.ActionGroupRemoveMember", "you have been removed from the group xxx by xxx")
		apidep.ClientManager.EnqueueMessage(uid, 0, notifyResp)
	}

	resp := message.NewMessage(ctx.Seq, "api.ActionSuccess", "remove member success")

	ctx.Response(resp)
	return nil
}

func (m *GroupApi) AddGroupMember(ctx *route.Context, request *AddMemberRequest) error {

	g, err := groupdao.GroupDao.GetGroup(request.Gid)
	if err != nil {
		return err
	}

	members, err := m.addGroupMember(g.Gid, request.Uid...)
	if err != nil {
		return err
	}

	// notify group member update group
	groupNotify := GroupAddMemberResponse{
		Gid:     g.Gid,
		Members: members,
	}
	apidep.GroupManager.DispatchNotifyMessage(g.Gid, message.NewMessage(-1, "api.ActionGroupAddMember", groupNotify))

	for _, member := range members {

		// add group to member's contacts list
		_, e := dao.UserDao.AddContacts(member.Uid, g.Gid, dao.ContactsTypeGroup, "")
		if e != nil {
			_ = apidep.GroupManager.RemoveMember(request.Gid, member.Uid)
			continue
		}
		//ms, err := groupdao.GroupDao.GetMembers(request.Gid)
		if err != nil {
			return err
		}
		//notify update contacts list
		//c := user.ContactResponse{
		//	Friends: []*user.InfoResponse{},
		//	Groups: []*GroupResponse{{
		//		Group:   *g,
		//		Members: ms,
		//	}},
		//}
		//respond(member.Uid, -1, "api.ActionUserAddFriend", c)

		// default add user chat
		chat, er := dao.ChatDao.NewUserChat(g.ChatId, member.Uid, g.Gid, dao.ChatTypeGroup)
		if er != nil {
			continue
		}
		// member subscribe group message
		apidep.GroupManager.PutMember(g.Gid, map[int64]int32{member.Uid: 1})

		// notify update chat list
		ctx.Response(message.NewMessage(-1, "", chat))
	}
	return nil
}

func (m *GroupApi) ExitGroup(ctx *route.Context, request *ExitGroupRequest) error {

	err := apidep.GroupManager.RemoveMember(request.Gid, ctx.Uid)
	if err != nil {
		return err
	}

	err = groupdao.GroupDao.RemoveMember(request.Gid, ctx.Uid)
	if err != nil {
		return err
	}
	resp := message.NewMessage(ctx.Seq, "api.ActionSuccess", "exit group success")
	ctx.Response(resp)
	return err
}

func (m *GroupApi) JoinGroup(ctx *route.Context, request *JoinGroupRequest) error {

	g, err := groupdao.GroupDao.GetGroup(request.Gid)
	if err != nil {
		return err
	}

	if g == nil {
		return errors.New("group does not exist")
	}

	_, err = m.addGroupMember(request.Gid, ctx.Uid)
	if err != nil {
		return err
	}

	_, err = dao.UserDao.AddContacts(ctx.Uid, g.Gid, dao.ContactsTypeGroup, "")

	//members, err := groupdao.GroupDao.GetMembers(request.Gid)
	//if err != nil {
	//	return err
	//}

	//c := user.ContactResponse{
	//	Friends: []*user.InfoResponse{},
	//	Groups: []*GroupResponse{{
	//		Group:   *g,
	//		Members: members,
	//	}},
	//}
	//respond(ctx.Uid, -1, "api.ActionUserAddFriend", c)

	chat, err := dao.ChatDao.NewUserChat(g.ChatId, ctx.Uid, g.Gid, dao.ChatTypeGroup)
	if err != nil {
		_ = groupdao.GroupDao.RemoveMember(request.Gid, ctx.Uid)
		return err
	}
	apidep.GroupManager.PutMember(g.Gid, map[int64]int32{ctx.Uid: 1})

	apidep.ClientManager.EnqueueMessage(ctx.Uid, ctx.Device, message.NewMessage(-1, "", chat))
	ctx.Response(message.NewMessage(ctx.Seq, "", "success"))
	return nil
}

func (m *GroupApi) createGroup(name string, uid int64) (*dao.Group, int64, error) {

	gp, err := groupdao.GroupDao.CreateGroup(name, uid)
	if err != nil {
		return nil, 0, err
	}
	// create group chat
	chat, err := dao.ChatDao.CreateChat(dao.ChatTypeGroup, 0, gp.Gid)
	if err != nil {
		// TODO undo
		return nil, 0, err
	}

	gp.ChatId = chat.Cid
	err = groupdao.GroupDao.UpdateGroupChatId(gp.Gid, chat.Cid)
	if err != nil {
		return nil, 0, err
	}

	_, err = groupdao.GroupDao.AddMember(gp.Gid, groupdao.GroupMemberAdmin, uid)
	if err != nil {
		// TODO undo create group
		return nil, 0, err
	}
	_, err = dao.UserDao.AddContacts(uid, gp.Gid, dao.ContactsTypeGroup, "")
	if err != nil {
		// TODO undo
		return nil, 0, err
	}
	apidep.GroupManager.AddGroup(gp.Gid)
	return gp, chat.Cid, nil
}

func (m *GroupApi) addGroupMember(gid int64, uid ...int64) ([]*dao.GroupMember, error) {

	memberUid := make([]int64, 0, len(uid))
	members, _ := groupdao.GroupDao.GetMember(gid, uid...)
	existsMember := map[int64]interface{}{}
	for _, i := range members {
		existsMember[i.Uid] = nil
	}

	for _, u := range uid {
		// member exist
		if _, ok := existsMember[u]; !ok {
			memberUid = append(memberUid, u)
		}
	}
	if len(memberUid) == 0 {
		return nil, errors.New("already added")
	}

	// TODO query user info and notify group members, optimize query time
	exist, err2 := dao.UserDao.HasUser(memberUid...)
	if err2 != nil {
		return nil, err2
	}
	if !exist {
		return nil, errors.New("user does not exist")
	}

	members, err := groupdao.GroupDao.AddMember(gid, groupdao.GroupMemberUser, memberUid...)
	if err != nil {
		return nil, err
	}
	return members, nil
}