package main

type approvalStatus string

const (
	approvalStatusPending  approvalStatus = "Pending"
	approvalStatusApproved approvalStatus = "Approved"
	approvalStatusDenied   approvalStatus = "Denied"
)
