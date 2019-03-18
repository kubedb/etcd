# Change Log

## [0.3.0](https://github.com/kubedb/etcd/tree/0.3.0) (2019-03-18)
[Full Changelog](https://github.com/kubedb/etcd/compare/0.2.0...0.3.0)

**Merged pull requests:**

- Keep the label-key-blacklist flag to run command only [\#61](https://github.com/kubedb/etcd/pull/61) ([tamalsaha](https://github.com/tamalsaha))
- Don't inherit app.kubernetes.io labels from CRD into offshoots [\#60](https://github.com/kubedb/etcd/pull/60) ([tamalsaha](https://github.com/tamalsaha))
- Add role label to stats service [\#59](https://github.com/kubedb/etcd/pull/59) ([tamalsaha](https://github.com/tamalsaha))
- Update Kubernetes client libraries to 1.13.0 [\#58](https://github.com/kubedb/etcd/pull/58) ([tamalsaha](https://github.com/tamalsaha))

## [0.2.0](https://github.com/kubedb/etcd/tree/0.2.0) (2019-02-19)
[Full Changelog](https://github.com/kubedb/etcd/compare/0.1.0...0.2.0)

**Merged pull requests:**

- Revendor dependencies [\#57](https://github.com/kubedb/etcd/pull/57) ([tamalsaha](https://github.com/tamalsaha))
- Revendor dependencies : Retry Failed Scheduler Snapshot [\#56](https://github.com/kubedb/etcd/pull/56) ([the-redback](https://github.com/the-redback))
- Revendor dependencies [\#55](https://github.com/kubedb/etcd/pull/55) ([the-redback](https://github.com/the-redback))
- Fix app binding [\#54](https://github.com/kubedb/etcd/pull/54) ([tamalsaha](https://github.com/tamalsaha))

## [0.1.0](https://github.com/kubedb/etcd/tree/0.1.0) (2018-12-17)
[Full Changelog](https://github.com/kubedb/etcd/compare/0.1.0-rc.2...0.1.0)

**Merged pull requests:**

- Reuse event recorder [\#53](https://github.com/kubedb/etcd/pull/53) ([tamalsaha](https://github.com/tamalsaha))
- Revendor dependencies [\#52](https://github.com/kubedb/etcd/pull/52) ([tamalsaha](https://github.com/tamalsaha))

## [0.1.0-rc.2](https://github.com/kubedb/etcd/tree/0.1.0-rc.2) (2018-12-06)
[Full Changelog](https://github.com/kubedb/etcd/compare/0.1.0-rc.1...0.1.0-rc.2)

**Merged pull requests:**

- Use flags.DumpAll [\#51](https://github.com/kubedb/etcd/pull/51) ([tamalsaha](https://github.com/tamalsaha))

## [0.1.0-rc.1](https://github.com/kubedb/etcd/tree/0.1.0-rc.1) (2018-12-02)
[Full Changelog](https://github.com/kubedb/etcd/compare/0.1.0-rc.0...0.1.0-rc.1)

**Merged pull requests:**

- Apply cleanup [\#50](https://github.com/kubedb/etcd/pull/50) ([tamalsaha](https://github.com/tamalsaha))
- Set periodic analytics [\#49](https://github.com/kubedb/etcd/pull/49) ([tamalsaha](https://github.com/tamalsaha))
- Fix analytics [\#48](https://github.com/kubedb/etcd/pull/48) ([the-redback](https://github.com/the-redback))
- Error out from backup cron job for deprecated db versions [\#47](https://github.com/kubedb/etcd/pull/47) ([the-redback](https://github.com/the-redback))
- Add CRDS without observation when operator starts [\#46](https://github.com/kubedb/etcd/pull/46) ([the-redback](https://github.com/the-redback))
- Introduce AppBinding support [\#41](https://github.com/kubedb/etcd/pull/41) ([tamalsaha](https://github.com/tamalsaha))

## [0.1.0-rc.0](https://github.com/kubedb/etcd/tree/0.1.0-rc.0) (2018-10-15)
[Full Changelog](https://github.com/kubedb/etcd/compare/0.1.0-beta.1...0.1.0-rc.0)

**Merged pull requests:**

- Add etcd and ugorji to glide.yaml [\#45](https://github.com/kubedb/etcd/pull/45) ([tamalsaha](https://github.com/tamalsaha))
- Various Fixes [\#44](https://github.com/kubedb/etcd/pull/44) ([hossainemruz](https://github.com/hossainemruz))
- Update kubernetes client libraries to 1.12.0 [\#43](https://github.com/kubedb/etcd/pull/43) ([tamalsaha](https://github.com/tamalsaha))
- Add validation webhook xray [\#42](https://github.com/kubedb/etcd/pull/42) ([tamalsaha](https://github.com/tamalsaha))
- Merge ports from service template [\#40](https://github.com/kubedb/etcd/pull/40) ([tamalsaha](https://github.com/tamalsaha))
- Replace doNotPause with TerminationPolicy = DoNotTerminate [\#39](https://github.com/kubedb/etcd/pull/39) ([tamalsaha](https://github.com/tamalsaha))
- Pass resources to NamespaceValidator [\#38](https://github.com/kubedb/etcd/pull/38) ([tamalsaha](https://github.com/tamalsaha))
- Various fixes [\#37](https://github.com/kubedb/etcd/pull/37) ([tamalsaha](https://github.com/tamalsaha))
- Support Livecycle hook and container probes [\#36](https://github.com/kubedb/etcd/pull/36) ([tamalsaha](https://github.com/tamalsaha))
- Check if Kubernetes version is supported before running operator [\#35](https://github.com/kubedb/etcd/pull/35) ([tamalsaha](https://github.com/tamalsaha))
- Update package alias [\#34](https://github.com/kubedb/etcd/pull/34) ([tamalsaha](https://github.com/tamalsaha))

## [0.1.0-beta.1](https://github.com/kubedb/etcd/tree/0.1.0-beta.1) (2018-09-30)
[Full Changelog](https://github.com/kubedb/etcd/compare/0.1.0-beta.0...0.1.0-beta.1)

**Merged pull requests:**

- Revendor api [\#33](https://github.com/kubedb/etcd/pull/33) ([tamalsaha](https://github.com/tamalsaha))
- Fix tests [\#32](https://github.com/kubedb/etcd/pull/32) ([tamalsaha](https://github.com/tamalsaha))
- Revendor api for catalog apigroup [\#31](https://github.com/kubedb/etcd/pull/31) ([tamalsaha](https://github.com/tamalsaha))
- Use --pull flag with docker build \(\#20\) [\#30](https://github.com/kubedb/etcd/pull/30) ([tamalsaha](https://github.com/tamalsaha))

## [0.1.0-beta.0](https://github.com/kubedb/etcd/tree/0.1.0-beta.0) (2018-09-20)
**Merged pull requests:**

- Show deprecated column for etcdversions [\#29](https://github.com/kubedb/etcd/pull/29) ([hossainemruz](https://github.com/hossainemruz))
- Support Termination Policy & Stop working for deprecated \*Versions. [\#28](https://github.com/kubedb/etcd/pull/28) ([the-redback](https://github.com/the-redback))
- Don't try to wipe out Snapshot data for Local backend  [\#27](https://github.com/kubedb/etcd/pull/27) ([hossainemruz](https://github.com/hossainemruz))
- Revendor k8s.io/apiserver [\#26](https://github.com/kubedb/etcd/pull/26) ([tamalsaha](https://github.com/tamalsaha))
- Revendor kubernetes-1.11.3 [\#25](https://github.com/kubedb/etcd/pull/25) ([tamalsaha](https://github.com/tamalsaha))
- Fix log formatting [\#24](https://github.com/kubedb/etcd/pull/24) ([tamalsaha](https://github.com/tamalsaha))
- Add TerminationPolicy for databases [\#23](https://github.com/kubedb/etcd/pull/23) ([tamalsaha](https://github.com/tamalsaha))
- Revendor api [\#22](https://github.com/kubedb/etcd/pull/22) ([tamalsaha](https://github.com/tamalsaha))
- Add travis.yml [\#21](https://github.com/kubedb/etcd/pull/21) ([tamalsaha](https://github.com/tamalsaha))
- Use IntHash as status.observedGeneration [\#20](https://github.com/kubedb/etcd/pull/20) ([tamalsaha](https://github.com/tamalsaha))
- Update status.ObservedGeneration for failure phase [\#19](https://github.com/kubedb/etcd/pull/19) ([the-redback](https://github.com/the-redback))
- Use NewObservableHandler [\#18](https://github.com/kubedb/etcd/pull/18) ([tamalsaha](https://github.com/tamalsaha))
- Introduce storageType : ephemeral [\#17](https://github.com/kubedb/etcd/pull/17) ([tamalsaha](https://github.com/tamalsaha))
- Support passing args via PodTemplate [\#16](https://github.com/kubedb/etcd/pull/16) ([tamalsaha](https://github.com/tamalsaha))
- Keep track of observedGeneration in status [\#15](https://github.com/kubedb/etcd/pull/15) ([tamalsaha](https://github.com/tamalsaha))
- Use UpdateEtcdStatus to update status [\#14](https://github.com/kubedb/etcd/pull/14) ([tamalsaha](https://github.com/tamalsaha))
- Etcd Issue fix [\#13](https://github.com/kubedb/etcd/pull/13) ([sanjid133](https://github.com/sanjid133))
- Revise immutable spec fields [\#12](https://github.com/kubedb/etcd/pull/12) ([tamalsaha](https://github.com/tamalsaha))
- Use updated crd spec [\#11](https://github.com/kubedb/etcd/pull/11) ([tamalsaha](https://github.com/tamalsaha))
- Use updated crd spec [\#10](https://github.com/kubedb/etcd/pull/10) ([tamalsaha](https://github.com/tamalsaha))
- Rename OffshootLabels to OffshootSelectors [\#9](https://github.com/kubedb/etcd/pull/9) ([tamalsaha](https://github.com/tamalsaha))
- Revendor api [\#8](https://github.com/kubedb/etcd/pull/8) ([tamalsaha](https://github.com/tamalsaha))
- Use kmodules monitoring and objectstore api [\#7](https://github.com/kubedb/etcd/pull/7) ([tamalsaha](https://github.com/tamalsaha))
- Uniq [\#6](https://github.com/kubedb/etcd/pull/6) ([sanjid133](https://github.com/sanjid133))
- Initial implementation [\#5](https://github.com/kubedb/etcd/pull/5) ([sanjid133](https://github.com/sanjid133))
- Don't panic if admission options is nil [\#4](https://github.com/kubedb/etcd/pull/4) ([tamalsaha](https://github.com/tamalsaha))
- Disable admission controllers for webhook server [\#3](https://github.com/kubedb/etcd/pull/3) ([tamalsaha](https://github.com/tamalsaha))
- Update client-go to 7.0.0 [\#2](https://github.com/kubedb/etcd/pull/2) ([tamalsaha](https://github.com/tamalsaha))
- Clone initial implementation from MongoDB [\#1](https://github.com/kubedb/etcd/pull/1) ([tamalsaha](https://github.com/tamalsaha))



\* *This Change Log was automatically generated by [github_changelog_generator](https://github.com/skywinder/Github-Changelog-Generator)*