package amipublisher

import (
	"errors"
	"github.com/Symantec/Dominator/lib/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"path"
	"strings"
	"time"
)

func createSession(accountProfileName string) (*session.Session, error) {
	return session.NewSessionWithOptions(session.Options{
		Profile:           accountProfileName,
		SharedConfigState: session.SharedConfigEnable})
}

func createService(awsSession *session.Session, regionName string) *ec2.EC2 {
	return ec2.New(awsSession, &aws.Config{Region: aws.String(regionName)})
}

func createVolume(awsService *ec2.EC2, availabilityZone *string, size uint64,
	tags map[string]string, logger log.Logger) (string, error) {
	// Strip out possible ExpiresAt tag.
	newTags := make(map[string]string)
	for key, value := range tags {
		switch key {
		case "ExpiresAt":
		case "Name":
		default:
			newTags[key] = value
		}
	}
	newTags["Name"] = "image unpacker"
	tags = newTags
	sizeInGiB := int64(size) >> 30
	if sizeInGiB<<30 < int64(size) {
		sizeInGiB++
	}
	volume, err := awsService.CreateVolume(&ec2.CreateVolumeInput{
		AvailabilityZone: availabilityZone,
		Encrypted:        aws.Bool(true),
		Size:             aws.Int64(sizeInGiB),
		VolumeType:       aws.String("gp2"),
	})
	if err != nil {
		return "", err
	}
	volumeIds := make([]string, 1)
	volumeIds[0] = *volume.VolumeId
	logger.Printf("Created: %s\n", *volume.VolumeId)
	if err := createTags(awsService, *volume.VolumeId, tags); err != nil {
		return "", err
	}
	logger.Printf("Tagged: %s, waiting...\n", *volume.VolumeId)
	for ; true; time.Sleep(time.Second) {
		desc, err := awsService.DescribeVolumes(&ec2.DescribeVolumesInput{
			VolumeIds: aws.StringSlice(volumeIds),
		})
		if err != nil {
			return "", err
		}
		logger.Printf("state: \"%s\"\n", *desc.Volumes[0].State)
		if *desc.Volumes[0].State == ec2.VolumeStateAvailable {
			break
		}
	}
	return *volume.VolumeId, nil
}

func createSnapshot(awsService *ec2.EC2, volumeId string,
	description string, tags map[string]string, logger log.Logger) (
	string, error) {
	snapshot, err := awsService.CreateSnapshot(&ec2.CreateSnapshotInput{
		VolumeId:    aws.String(volumeId),
		Description: aws.String(description),
	})
	if err != nil {
		return "", err
	}
	snapshotIds := make([]string, 1)
	snapshotIds[0] = *snapshot.SnapshotId
	logger.Printf("Created: %s\n", *snapshot.SnapshotId)
	// Strip out possible Name tag.
	newTags := make(map[string]string)
	for key, value := range tags {
		switch key {
		case "Name":
		default:
			newTags[key] = value
		}
	}
	newTags["Name"] = description
	tags = newTags
	if err := createTags(awsService, *snapshot.SnapshotId, tags); err != nil {
		return "", err
	}
	logger.Printf("Tagged: %s, waiting...\n", *snapshot.SnapshotId)
	for ; true; time.Sleep(time.Second * 15) {
		desc, err := awsService.DescribeSnapshots(&ec2.DescribeSnapshotsInput{
			SnapshotIds: aws.StringSlice(snapshotIds),
		})
		if err != nil {
			return "", err
		}
		logger.Printf("state: \"%s\"\n", *desc.Snapshots[0].State)
		if *desc.Snapshots[0].State == ec2.SnapshotStateCompleted {
			break
		}
	}
	return *snapshot.SnapshotId, nil
}

func registerAmi(awsService *ec2.EC2, snapshotId string, amiName string,
	imageName string, tags map[string]string, logger log.Logger) (
	string, error) {
	rootDevName := "/dev/sda1"
	blkDevMaps := make([]*ec2.BlockDeviceMapping, 1)
	blkDevMaps[0] = &ec2.BlockDeviceMapping{
		DeviceName: aws.String(rootDevName),
		Ebs: &ec2.EbsBlockDevice{
			DeleteOnTermination: aws.Bool(true),
			SnapshotId:          aws.String(snapshotId),
			VolumeType:          aws.String("gp2"),
		},
	}
	if amiName == "" {
		amiName = imageName
	}
	amiName = strings.Replace(amiName, ":", ".", -1)
	ami, err := awsService.RegisterImage(&ec2.RegisterImageInput{
		Architecture:        aws.String("x86_64"),
		BlockDeviceMappings: blkDevMaps,
		Description:         aws.String(imageName),
		Name:                aws.String(amiName),
		RootDeviceName:      aws.String(rootDevName),
		VirtualizationType:  aws.String("hvm"),
	})
	if err != nil {
		return "", err
	}
	logger.Printf("Created: %s\n", *ami.ImageId)
	imageIds := make([]string, 1)
	imageIds[0] = *ami.ImageId
	// Strip out possible Name tag.
	newTags := make(map[string]string)
	for key, value := range tags {
		switch key {
		case "Name":
		default:
			newTags[key] = value
		}
	}
	newTags["Name"] = path.Dir(imageName)
	tags = newTags
	if err := createTags(awsService, *ami.ImageId, tags); err != nil {
		return "", err
	}
	logger.Printf("Tagged: %s, waiting...\n", *ami.ImageId)
	for ; true; time.Sleep(time.Second) {
		desc, err := awsService.DescribeImages(&ec2.DescribeImagesInput{
			ImageIds: aws.StringSlice(imageIds),
		})
		if err != nil {
			return "", err
		}
		logger.Printf("state: \"%s\"\n", *desc.Images[0].State)
		if *desc.Images[0].State == ec2.ImageStateAvailable {
			break
		}
	}
	return *ami.ImageId, nil
}

func createTags(awsService *ec2.EC2, resourceId string,
	tags map[string]string) error {
	resourceIds := make([]string, 1)
	resourceIds[0] = resourceId
	awsTags := make([]*ec2.Tag, 0, len(tags))
	for key, value := range tags {
		awsTags = append(awsTags,
			&ec2.Tag{Key: aws.String(key), Value: aws.String(value)})
	}
	_, err := awsService.CreateTags(&ec2.CreateTagsInput{
		Resources: aws.StringSlice(resourceIds),
		Tags:      awsTags,
	})
	return err
}

func attachVolume(awsService *ec2.EC2, instanceId string, volumeId string,
	logger log.Logger) error {
	instanceIds := make([]string, 1)
	instanceIds[0] = instanceId
	desc, err := awsService.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice(instanceIds),
	})
	if err != nil {
		return err
	}
	usedBlockDevices := make(map[string]struct{})
	instance := desc.Reservations[0].Instances[0]
	for _, device := range instance.BlockDeviceMappings {
		usedBlockDevices[aws.StringValue(device.DeviceName)] = struct{}{}
	}
	var blockDeviceName string
	for c := 'f'; c <= 'p'; c++ {
		name := "/dev/sd" + string(c)
		if _, ok := usedBlockDevices[name]; !ok {
			blockDeviceName = name
			break
		}
	}
	if blockDeviceName == "" {
		return errors.New("no space for new block device")
	}
	_, err = awsService.AttachVolume(&ec2.AttachVolumeInput{
		Device:     aws.String(blockDeviceName),
		InstanceId: aws.String(instanceId),
		VolumeId:   aws.String(volumeId),
	})
	if err != nil {
		return err
	}
	blockDevMappings := make([]*ec2.InstanceBlockDeviceMappingSpecification, 1)
	blockDevMappings[0] = &ec2.InstanceBlockDeviceMappingSpecification{
		DeviceName: aws.String(blockDeviceName),
		Ebs: &ec2.EbsInstanceBlockDeviceSpecification{
			DeleteOnTermination: aws.Bool(true),
			VolumeId:            aws.String(volumeId),
		},
	}
	_, err = awsService.ModifyInstanceAttribute(
		&ec2.ModifyInstanceAttributeInput{
			BlockDeviceMappings: blockDevMappings,
			InstanceId:          aws.String(instanceId),
		})
	if err != nil {
		return err
	}
	logger.Printf("Requested attach(%s): %s on %s, waiting...\n",
		instanceId, volumeId, blockDeviceName)
	volumeIds := make([]string, 1)
	volumeIds[0] = volumeId
	for ; true; time.Sleep(time.Second) {
		desc, err := awsService.DescribeVolumes(&ec2.DescribeVolumesInput{
			VolumeIds: aws.StringSlice(volumeIds),
		})
		if err != nil {
			return err
		}
		state := *desc.Volumes[0].Attachments[0].State
		logger.Printf("state: \"%s\"\n", state)
		if state == ec2.VolumeAttachmentStateAttached {
			break
		}
	}
	logger.Printf("Attached: %s\n", volumeId)
	return nil
}

func listRegions(awsService *ec2.EC2) ([]string, error) {
	out, err := awsService.DescribeRegions(&ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, err
	}
	regionNames := make([]string, 0, len(out.Regions))
	for _, region := range out.Regions {
		regionNames = append(regionNames, aws.StringValue(region.RegionName))
	}
	return regionNames, nil
}

func getInstances(awsService *ec2.EC2, nameTag string) (
	[]*ec2.Instance, error) {
	tagValues := make([]string, 1)
	tagValues[0] = nameTag
	filters := make([]*ec2.Filter, 1)
	filters[0] = &ec2.Filter{
		Name:   aws.String("tag:Name"),
		Values: aws.StringSlice(tagValues),
	}
	out, err := awsService.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}
	instances := make([]*ec2.Instance, 0)
	for _, reservation := range out.Reservations {
		for _, instance := range reservation.Instances {
			instances = append(instances, instance)
		}
	}
	return instances, nil
}
