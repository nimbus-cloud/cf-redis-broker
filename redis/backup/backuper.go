package backup

import (
	"io/ioutil"
	"path/filepath"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/pivotal-cf/cf-redis-broker/recovery"
	"github.com/pivotal-cf/cf-redis-broker/recovery/task"
	redis "github.com/pivotal-cf/cf-redis-broker/redis/client"
	"github.com/pivotal-golang/lager"
)

type RedisSnapshotterProvider func(
	client redis.Client,
	timeout time.Duration,
	logger lager.Logger,
) recovery.Snapshotter

type RenameTaskProvider func(
	targerPath string,
	logger lager.Logger,
) task.Task

type S3UploadTaskProvider func(
	bucketName string,
	targetPath string,
	endpoint string,
	accessKey string,
	secretKey string,
	logger lager.Logger,
	injectors ...task.S3UploadInjector,
) task.Task

type CleanupTaskProvider func(
	originalRdbPath string,
	renamedRdbPath string,
	logger lager.Logger,
	injectors ...CleanupInjector,
) task.Task

type BackupInjector func(*backuper)

type backuper struct {
	snapshotterProvider  RedisSnapshotterProvider
	renameTaskProvider   RenameTaskProvider
	s3UploadTaskProvider S3UploadTaskProvider
	cleanupTaskProvider  CleanupTaskProvider

	snapshotTimeout time.Duration
	s3BucketName    string
	s3Endpoint      string
	awsAccessKey    string
	awsSecretKey    string
	logger          lager.Logger
}

type RedisBackuper interface {
	Backup(redis.Client, string) error
}

func NewRedisBackuper(
	snapshotTimeout time.Duration,
	s3BucketName string,
	s3Endpoint string,
	awsAccessKey string,
	awsSecretKey string,
	logger lager.Logger,
	injectors ...BackupInjector,
) *backuper {
	backuper := &backuper{
		snapshotterProvider:  NewSnapshotter,
		renameTaskProvider:   task.NewRename,
		s3UploadTaskProvider: task.NewS3Upload,
		cleanupTaskProvider:  NewCleanup,
		snapshotTimeout:      snapshotTimeout,
		s3BucketName:         s3BucketName,
		s3Endpoint:           s3Endpoint,
		awsAccessKey:         awsAccessKey,
		awsSecretKey:         awsSecretKey,
		logger:               logger,
	}

	for _, injector := range injectors {
		injector(backuper)
	}

	return backuper
}

func (b *backuper) Backup(client redis.Client, s3TargetPath string) error {
	localLogger := b.logger.WithData(lager.Data{
		"redis_address": client.Address(),
	})

	localLogger.Info("backup", lager.Data{"event": "starting"})

	snapshotter := b.snapshotterProvider(client, b.snapshotTimeout, b.logger)
	artifact, err := snapshotter.Snapshot()
	if err != nil {
		localLogger.Error("backup", err, lager.Data{"event": "failed"})
		return err
	}

	originalPath := artifact.Path()
	tmpDir, err := ioutil.TempDir("", "redis-backup")
	if err != nil {
		localLogger.Error("backup", err, lager.Data{"event": "failed"})
		return err
	}

	tmpSnapshotPath := filepath.Join(tmpDir, uuid.New())

	renameTask := b.renameTaskProvider(tmpSnapshotPath, b.logger)

	uploadTask := b.s3UploadTaskProvider(
		b.s3BucketName,
		s3TargetPath,
		b.s3Endpoint,
		b.awsAccessKey,
		b.awsSecretKey,
		b.logger,
	)

	cleanupTask := b.cleanupTaskProvider(
		originalPath,
		tmpSnapshotPath,
		b.logger,
	)

	artifact, err = task.NewPipeline(
		"redis-backup",
		b.logger,
		renameTask,
		uploadTask,
	).Run(artifact)

	if err != nil {
		localLogger.Error("backup", err, lager.Data{"event": "failed"})
	}

	task.NewPipeline(
		"cleanup",
		b.logger,
		cleanupTask,
	).Run(artifact)

	localLogger.Info("backup", lager.Data{"event": "done"})

	return err
}

func InjectSnapshotterProvider(provider RedisSnapshotterProvider) BackupInjector {
	return func(b *backuper) {
		b.snapshotterProvider = provider
	}
}

func InjectRenameTaskProvider(provider RenameTaskProvider) BackupInjector {
	return func(b *backuper) {
		b.renameTaskProvider = provider
	}
}

func InjectS3UploadTaskProvider(provider S3UploadTaskProvider) BackupInjector {
	return func(b *backuper) {
		b.s3UploadTaskProvider = provider
	}
}

func InjectCleanupTaskProvider(provider CleanupTaskProvider) BackupInjector {
	return func(b *backuper) {
		b.cleanupTaskProvider = provider
	}
}
