package e2e_test

import (
	"fmt"
	"os"

	"github.com/appscode/go/types"
	meta_util "github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/etcd/test/e2e/framework"
	"github.com/kubedb/etcd/test/e2e/matcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	S3_BUCKET_NAME       = "S3_BUCKET_NAME"
	GCS_BUCKET_NAME      = "GCS_BUCKET_NAME"
	AZURE_CONTAINER_NAME = "AZURE_CONTAINER_NAME"
	SWIFT_CONTAINER_NAME = "SWIFT_CONTAINER_NAME"
)

var _ = Describe("Etcd", func() {
	var (
		err         error
		f           *framework.Invocation
		etcd        *api.Etcd
		snapshot    *api.Snapshot
		secret      *core.Secret
		skipMessage string
	)

	BeforeEach(func() {
		f = root.Invoke()
		etcd = f.Etcd()
		snapshot = f.Snapshot()
		skipMessage = ""
	})

	var createAndWaitForRunning = func() {
		By("Create Etcd: " + etcd.Name)
		err = f.CreateEtcd(etcd)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running etcd")
		f.EventuallyEtcdRunning(etcd.ObjectMeta).Should(BeTrue())
	}

	var deleteTestResource = func() {
		By("Delete etcd")
		err = f.DeleteEtcd(etcd.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for etcd to be paused")
		f.EventuallyDormantDatabaseStatus(etcd.ObjectMeta).Should(matcher.HavePaused())

		By("Set DormantDatabase Spec.WipeOut to true")
		_, err := f.PatchDormantDatabase(etcd.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
			in.Spec.WipeOut = true
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		By("Delete Dormant Database")
		err = f.DeleteDormantDatabase(etcd.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for etcd resources to be wipedOut")
		f.EventuallyWipedOut(etcd.ObjectMeta).Should(Succeed())
	}

	var shouldSuccessfullyRunning = func() {
		if skipMessage != "" {
			Skip(skipMessage)
		}

		// Create Etcd
		createAndWaitForRunning()

		// Delete test resource
		deleteTestResource()
	}

	Describe("Test", func() {
		BeforeEach(func() {
			if f.StorageClass == "" {
				Skip("Missing StorageClassName. Provide as flag to test this.")
			}
			etcd.Spec.Storage = &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				StorageClassName: types.StringP(f.StorageClass),
			}

		})

		Context("General", func() {

			Context("Without PVC", func() {
				BeforeEach(func() {
					etcd.Spec.Storage = nil
				})
				It("should run successfully", shouldSuccessfullyRunning)
			})

			Context("With PVC", func() {
				It("should run successfully", func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}
					// Create MySQL
					createAndWaitForRunning()

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(etcd.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					By("Delete etcd")
					err = f.DeleteEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for etcd to be paused")
					f.EventuallyDormantDatabaseStatus(etcd.ObjectMeta).Should(matcher.HavePaused())

					// Create Etcd object again to resume it
					By("Create Etcd: " + etcd.Name)
					err = f.CreateEtcd(etcd)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(etcd.ObjectMeta).Should(BeFalse())

					By("Wait for Running etcd")
					f.EventuallyEtcdRunning(etcd.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					deleteTestResource()
				})
			})
		})

		Context("DoNotPause", func() {
			BeforeEach(func() {
				etcd.Spec.DoNotPause = true
			})

			It("should work successfully", func() {
				// Create and wait for running Etcd
				createAndWaitForRunning()

				By("Delete etcd")
				err = f.DeleteEtcd(etcd.ObjectMeta)
				Expect(err).Should(HaveOccurred())

				By("Etcd is not paused. Check for etcd")
				f.EventuallyEtcd(etcd.ObjectMeta).Should(BeTrue())

				By("Check for Running etcd")
				f.EventuallyEtcdRunning(etcd.ObjectMeta).Should(BeTrue())

				By("Update etcd to set DoNotPause=false")
				f.PatchEtcd(etcd.ObjectMeta, func(in *api.Etcd) *api.Etcd {
					in.Spec.DoNotPause = false
					return in
				})

				// Delete test resource
				deleteTestResource()
			})
		})

		Context("Snapshot", func() {
			var skipDataCheck bool

			AfterEach(func() {
				f.DeleteSecret(secret.ObjectMeta)
			})

			BeforeEach(func() {
				skipDataCheck = false
				snapshot.Spec.DatabaseName = etcd.Name
			})

			var shouldTakeSnapshot = func() {
				// Create and wait for running Etcd
				createAndWaitForRunning()

				By("Create Secret")
				err := f.CreateSecret(secret)
				Expect(err).NotTo(HaveOccurred())

				By("Create Snapshot")
				err = f.CreateSnapshot(snapshot)
				Expect(err).NotTo(HaveOccurred())

				By("Check for Succeeded snapshot")
				f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

				if !skipDataCheck {
					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
				}

				// Delete test resource
				deleteTestResource()

				if !skipDataCheck {
					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeFalse())
				}
			}

			Context("In Local", func() {
				BeforeEach(func() {
					skipDataCheck = true
					secret = f.SecretForLocalBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Local = &api.LocalSpec{
						MountPath: "/repo",
						VolumeSource: core.VolumeSource{
							EmptyDir: &core.EmptyDirVolumeSource{},
						},
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)

				// Additional
				Context("Without PVC", func() {
					BeforeEach(func() {
						etcd.Spec.Storage = nil
					})
					It("should run successfully", shouldTakeSnapshot)
				})
			})

			Context("In S3", func() {
				BeforeEach(func() {
					secret = f.SecretForS3Backend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.S3 = &api.S3Spec{
						Bucket: os.Getenv(S3_BUCKET_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In GCS", func() {
				BeforeEach(func() {
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &api.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
				})

				Context("Without Init", func() {
					It("should take Snapshot successfully", shouldTakeSnapshot)
				})

				Context("With Init", func() {
					BeforeEach(func() {
						etcd.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/etcd-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})

					It("should take Snapshot successfully", shouldTakeSnapshot)
				})

				Context("Delete One Snapshot keeping others", func() {
					BeforeEach(func() {
						etcd.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/etcd-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})

					It("Delete One Snapshot keeping others", func() {
						// Create and wait for running Etcd
						createAndWaitForRunning()

						By("Create Secret")
						err := f.CreateSecret(secret)
						Expect(err).NotTo(HaveOccurred())

						By("Create Snapshot")
						err = f.CreateSnapshot(snapshot)
						Expect(err).NotTo(HaveOccurred())

						By("Check for Succeeded snapshot")
						f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

						if !skipDataCheck {
							By("Check for snapshot data")
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}

						oldSnapshot := snapshot

						// create new Snapshot
						snapshot := f.Snapshot()
						snapshot.Spec.DatabaseName = etcd.Name
						snapshot.Spec.StorageSecretName = secret.Name
						snapshot.Spec.GCS = &api.GCSSpec{
							Bucket: os.Getenv(GCS_BUCKET_NAME),
						}

						By("Create Snapshot")
						err = f.CreateSnapshot(snapshot)
						Expect(err).NotTo(HaveOccurred())

						By("Check for Succeeded snapshot")
						f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

						if !skipDataCheck {
							By("Check for snapshot data")
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}

						By(fmt.Sprintf("Delete Snapshot %v", snapshot.Name))
						err = f.DeleteSnapshot(snapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for Deleting Snapshot")
						f.EventuallySnapshot(etcd.ObjectMeta).Should(BeFalse())
						if !skipDataCheck {
							By("Check for snapshot data")
							f.EventuallySnapshotDataFound(snapshot).Should(BeFalse())
						}

						snapshot = oldSnapshot

						By(fmt.Sprintf("Old Snapshot %v Still Exists", snapshot.Name))
						_, err = f.GetSnapshot(snapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						if !skipDataCheck {
							By(fmt.Sprintf("Check for old snapshot %v data", snapshot.Name))
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}

						// Delete test resource
						deleteTestResource()

						if !skipDataCheck {
							By("Check for snapshot data")
							f.EventuallySnapshotDataFound(snapshot).Should(BeFalse())
						}
					})
				})

			})

			Context("In Azure", func() {
				BeforeEach(func() {
					secret = f.SecretForAzureBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Azure = &api.AzureSpec{
						Container: os.Getenv(AZURE_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In Swift", func() {
				BeforeEach(func() {
					secret = f.SecretForSwiftBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Swift = &api.SwiftSpec{
						Container: os.Getenv(SWIFT_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})
		})

		Context("Initialize", func() {
			Context("With Script", func() {
				BeforeEach(func() {
					etcd.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								GitRepo: &core.GitRepoVolumeSource{
									Repository: "https://github.com/kubedb/etcd-init-scripts.git",
									Directory:  ".",
								},
							},
						},
					}
				})

				It("should run successfully", func() {
					// Create Postgres
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					// Delete test resource
					deleteTestResource()
				})

			})

			Context("With Snapshot", func() {
				AfterEach(func() {
					f.DeleteSecret(secret.ObjectMeta)
				})

				BeforeEach(func() {
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &api.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
					snapshot.Spec.DatabaseName = etcd.Name
				})

				It("should run successfully", func() {
					// Create and wait for running Etcd
					createAndWaitForRunning()

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(etcd.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					By("Create Secret")
					f.CreateSecret(secret)

					By("Create Snapshot")
					f.CreateSnapshot(snapshot)

					By("Check for Succeeded snapshot")
					f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())

					oldEtcd, err := f.GetEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Create etcd from snapshot")
					etcd = f.Etcd()
					if f.StorageClass != "" {
						etcd.Spec.Storage = &core.PersistentVolumeClaimSpec{
							Resources: core.ResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
							StorageClassName: types.StringP(f.StorageClass),
						}
					}
					etcd.Spec.Init = &api.InitSpec{
						SnapshotSource: &api.SnapshotSourceSpec{
							Namespace: snapshot.Namespace,
							Name:      snapshot.Name,
						},
					}

					// Create and wait for running Etcd
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					// Delete test resource
					deleteTestResource()
					etcd = oldEtcd
					// Delete test resource
					deleteTestResource()
				})
			})
		})

		Context("Resume", func() {
			var usedInitScript bool
			var usedInitSnapshot bool
			BeforeEach(func() {
				usedInitScript = false
				usedInitSnapshot = false
			})

			Context("Super Fast User - Create-Delete-Create-Delete-Create ", func() {
				It("should resume DormantDatabase successfully", func() {
					// Create and wait for running Etcd
					createAndWaitForRunning()

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(etcd.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					By("Delete etcd")
					err = f.DeleteEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for etcd to be paused")
					f.EventuallyDormantDatabaseStatus(etcd.ObjectMeta).Should(matcher.HavePaused())

					// Create Etcd object again to resume it
					By("Create Etcd: " + etcd.Name)
					err = f.CreateEtcd(etcd)
					Expect(err).NotTo(HaveOccurred())

					// Delete without caring if DB is resumed
					By("Delete etcd")
					err = f.DeleteEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Create Etcd object again to resume it
					By("Create Etcd: " + etcd.Name)
					err = f.CreateEtcd(etcd)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(etcd.ObjectMeta).Should(BeFalse())

					By("Wait for Running etcd")
					f.EventuallyEtcdRunning(etcd.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					_, err = f.GetEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Delete test resource
					deleteTestResource()
				})
			})

			Context("Without Init", func() {
				It("should resume DormantDatabase successfully", func() {
					// Create and wait for running Etcd
					createAndWaitForRunning()

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(etcd.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					By("Delete etcd")
					err = f.DeleteEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for etcd to be paused")
					f.EventuallyDormantDatabaseStatus(etcd.ObjectMeta).Should(matcher.HavePaused())

					// Create Etcd object again to resume it
					By("Create Etcd: " + etcd.Name)
					err = f.CreateEtcd(etcd)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(etcd.ObjectMeta).Should(BeFalse())

					By("Wait for Running etcd")
					f.EventuallyEtcdRunning(etcd.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					_, err = f.GetEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Delete test resource
					deleteTestResource()
				})
			})

			Context("with init Script", func() {
				BeforeEach(func() {
					usedInitScript = true
					etcd.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								GitRepo: &core.GitRepoVolumeSource{
									Repository: "https://github.com/kubedb/etcd-init-scripts.git",
									Directory:  ".",
								},
							},
						},
					}
				})

				It("should resume DormantDatabase successfully", func() {
					// Create and wait for running Etcd
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					By("Delete etcd")
					err = f.DeleteEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for etcd to be paused")
					f.EventuallyDormantDatabaseStatus(etcd.ObjectMeta).Should(matcher.HavePaused())

					// Create Etcd object again to resume it
					By("Create Etcd: " + etcd.Name)
					err = f.CreateEtcd(etcd)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(etcd.ObjectMeta).Should(BeFalse())

					By("Wait for Running etcd")
					f.EventuallyEtcdRunning(etcd.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					_, err := f.GetEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Delete test resource
					deleteTestResource()
					if usedInitScript {
						Expect(etcd.Spec.Init).ShouldNot(BeNil())
						if usedInitScript {
							Expect(etcd.Spec.Init).ShouldNot(BeNil())
							_, err := meta_util.GetString(etcd.Annotations, api.AnnotationInitialized)
							Expect(err).To(HaveOccurred())
						}
					}
				})
			})

			Context("With Snapshot Init", func() {
				var skipDataCheck bool
				AfterEach(func() {
					f.DeleteSecret(secret.ObjectMeta)
				})
				BeforeEach(func() {
					skipDataCheck = false
					usedInitSnapshot = true
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &api.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
					snapshot.Spec.DatabaseName = etcd.Name
				})
				It("should resume successfully", func() {
					// Create and wait for running Etcd
					createAndWaitForRunning()

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(etcd.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					By("Create Secret")
					f.CreateSecret(secret)

					By("Create Snapshot")
					f.CreateSnapshot(snapshot)

					By("Check for Succeeded snapshot")
					f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())

					oldEtcd, err := f.GetEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Create etcd from snapshot")
					etcd = f.Etcd()
					if f.StorageClass != "" {
						etcd.Spec.Storage = &core.PersistentVolumeClaimSpec{
							Resources: core.ResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
							StorageClassName: types.StringP(f.StorageClass),
						}
					}
					etcd.Spec.Init = &api.InitSpec{
						SnapshotSource: &api.SnapshotSourceSpec{
							Namespace: snapshot.Namespace,
							Name:      snapshot.Name,
						},
					}

					// Create and wait for running Etcd
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					By("Delete etcd")
					err = f.DeleteEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for etcd to be paused")
					f.EventuallyDormantDatabaseStatus(etcd.ObjectMeta).Should(matcher.HavePaused())

					// Create Etcd object again to resume it
					By("Create Etcd: " + etcd.Name)
					err = f.CreateEtcd(etcd)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(etcd.ObjectMeta).Should(BeFalse())

					By("Wait for Running etcd")
					f.EventuallyEtcdRunning(etcd.ObjectMeta).Should(BeTrue())

					etcd, err = f.GetEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					if usedInitSnapshot {
						Expect(etcd.Spec.Init).ShouldNot(BeNil())
						_, err := meta_util.GetString(etcd.Annotations, api.AnnotationInitialized)
						Expect(err).NotTo(HaveOccurred())
					}

					// Delete test resource
					deleteTestResource()
					etcd = oldEtcd
					// Delete test resource
					deleteTestResource()
					if !skipDataCheck {
						By("Check for snapshot data")
						f.EventuallySnapshotDataFound(snapshot).Should(BeFalse())
					}
				})
			})

			Context("Multiple times with init script", func() {
				BeforeEach(func() {
					usedInitScript = true
					etcd.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								GitRepo: &core.GitRepoVolumeSource{
									Repository: "https://github.com/kubedb/etcd-init-scripts.git",
									Directory:  ".",
								},
							},
						},
					}
				})

				It("should resume DormantDatabase successfully", func() {
					// Create and wait for running Etcd
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					for i := 0; i < 3; i++ {
						By(fmt.Sprintf("%v-th", i+1) + " time running.")
						By("Delete etcd")
						err = f.DeleteEtcd(etcd.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for etcd to be paused")
						f.EventuallyDormantDatabaseStatus(etcd.ObjectMeta).Should(matcher.HavePaused())

						// Create Etcd object again to resume it
						By("Create Etcd: " + etcd.Name)
						err = f.CreateEtcd(etcd)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for DormantDatabase to be deleted")
						f.EventuallyDormantDatabase(etcd.ObjectMeta).Should(BeFalse())

						By("Wait for Running etcd")
						f.EventuallyEtcdRunning(etcd.ObjectMeta).Should(BeTrue())

						_, err := f.GetEtcd(etcd.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Checking Inserted Document")
						f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

						if usedInitScript {
							Expect(etcd.Spec.Init).ShouldNot(BeNil())
							_, err := meta_util.GetString(etcd.Annotations, api.AnnotationInitialized)
							Expect(err).To(HaveOccurred())
						}
					}

					// Delete test resource
					deleteTestResource()
				})
			})

		})

		Context("SnapshotScheduler", func() {
			AfterEach(func() {
				f.DeleteSecret(secret.ObjectMeta)
			})

			Context("With Startup", func() {

				var shouldStartupSchedular = func() {
					By("Create Secret")
					f.CreateSecret(secret)

					// Create and wait for running Etcd
					createAndWaitForRunning()

					By("Count multiple Snapshot Object")
					f.EventuallySnapshotCount(etcd.ObjectMeta).Should(matcher.MoreThan(3))

					By("Remove Backup Scheduler from Etcd")
					_, err = f.PatchEtcd(etcd.ObjectMeta, func(in *api.Etcd) *api.Etcd {
						in.Spec.BackupSchedule = nil
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Verify multiple Succeeded Snapshot")
					f.EventuallyMultipleSnapshotFinishedProcessing(etcd.ObjectMeta).Should(Succeed())

					deleteTestResource()
				}

				Context("with local", func() {
					BeforeEach(func() {
						secret = f.SecretForLocalBackend()
						etcd.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 1m",
							SnapshotStorageSpec: api.SnapshotStorageSpec{
								StorageSecretName: secret.Name,
								Local: &api.LocalSpec{
									MountPath: "/repo",
									VolumeSource: core.VolumeSource{
										EmptyDir: &core.EmptyDirVolumeSource{},
									},
								},
							},
						}
					})

					It("should run schedular successfully", shouldStartupSchedular)
				})

				Context("with GCS and PVC", func() {
					BeforeEach(func() {
						secret = f.SecretForGCSBackend()
						etcd.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 1m",
							SnapshotStorageSpec: api.SnapshotStorageSpec{
								StorageSecretName: secret.Name,
								GCS: &api.GCSSpec{
									Bucket: os.Getenv(GCS_BUCKET_NAME),
								},
							},
						}
					})

					It("should run schedular successfully", shouldStartupSchedular)
				})
			})

			Context("With Update - with Local", func() {
				BeforeEach(func() {
					secret = f.SecretForLocalBackend()
				})
				It("should run schedular successfully", func() {
					// Create and wait for running Etcd
					createAndWaitForRunning()

					By("Create Secret")
					f.CreateSecret(secret)

					By("Update etcd")
					_, err = f.PatchEtcd(etcd.ObjectMeta, func(in *api.Etcd) *api.Etcd {
						in.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 1m",
							SnapshotStorageSpec: api.SnapshotStorageSpec{
								StorageSecretName: secret.Name,
								Local: &api.LocalSpec{
									MountPath: "/repo",
									VolumeSource: core.VolumeSource{
										EmptyDir: &core.EmptyDirVolumeSource{},
									},
								},
							},
						}

						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Count multiple Snapshot Object")
					f.EventuallySnapshotCount(etcd.ObjectMeta).Should(matcher.MoreThan(3))

					By("Remove Backup Scheduler from Etcd")
					_, err = f.PatchEtcd(etcd.ObjectMeta, func(in *api.Etcd) *api.Etcd {
						in.Spec.BackupSchedule = nil
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Verify multiple Succeeded Snapshot")
					f.EventuallyMultipleSnapshotFinishedProcessing(etcd.ObjectMeta).Should(Succeed())

					deleteTestResource()
				})
			})

			Context("Re-Use DormantDatabase's scheduler", func() {
				BeforeEach(func() {
					secret = f.SecretForLocalBackend()
				})
				It("should re-use scheduler successfully", func() {
					// Create and wait for running Etcd
					createAndWaitForRunning()

					By("Create Secret")
					f.CreateSecret(secret)

					By("Update etcd")
					_, err = f.PatchEtcd(etcd.ObjectMeta, func(in *api.Etcd) *api.Etcd {
						in.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 1m",
							SnapshotStorageSpec: api.SnapshotStorageSpec{
								StorageSecretName: secret.Name,
								Local: &api.LocalSpec{
									MountPath: "/repo",
									VolumeSource: core.VolumeSource{
										EmptyDir: &core.EmptyDirVolumeSource{},
									},
								},
							},
						}
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(etcd.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					By("Count multiple Snapshot Object")
					f.EventuallySnapshotCount(etcd.ObjectMeta).Should(matcher.MoreThan(3))

					By("Delete etcd")
					err = f.DeleteEtcd(etcd.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for etcd to be paused")
					f.EventuallyDormantDatabaseStatus(etcd.ObjectMeta).Should(matcher.HavePaused())

					// Create Etcd object again to resume it
					By("Create Etcd: " + etcd.Name)
					err = f.CreateEtcd(etcd)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(etcd.ObjectMeta).Should(BeFalse())

					By("Wait for Running etcd")
					f.EventuallyEtcdRunning(etcd.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(etcd.ObjectMeta).Should(BeTrue())

					By("Count multiple Snapshot Object")
					f.EventuallySnapshotCount(etcd.ObjectMeta).Should(matcher.MoreThan(5))

					By("Remove Backup Scheduler from Etcd")
					_, err = f.PatchEtcd(etcd.ObjectMeta, func(in *api.Etcd) *api.Etcd {
						in.Spec.BackupSchedule = nil
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Verify multiple Succeeded Snapshot")
					f.EventuallyMultipleSnapshotFinishedProcessing(etcd.ObjectMeta).Should(Succeed())

					deleteTestResource()
				})
			})
		})
	})
})
