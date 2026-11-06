# Design

## Context

Sources are the DSM 7 Developer Guide (DSM 7.2.2 edition), read on 2026-11-06: the publication review checklist (Appendix C), INFO fields, scripts, FHS, privilege, and the port-config, data-share and config-update pages. Behaviour described below comes from the guide and hasn't been observed on a NAS unless a task says so.

## Decisions

### D1. Port registration through port-config
Review items 15 and 16 ask for every listening port to be registered. The guide's worker copies a `.sc` file from the package's target folder after `postinst` and removes it after `postuninst`. The port is chosen in the install wizard, so `postinst` writes the file on every install and upgrade from `settings.env`, and the build ships one for 8580 as a fallback. `port_forward` is `no`, because the port only listens on 127.0.0.1. A port changed by hand in `settings.env` reaches the firewall at the next upgrade; calling `synopkghelper update` from the app isn't worth it for that case.

### D2. Uninstall asks, Keep is the default
The guide's FHS table says the `var` folder is not removed on uninstall, while review item 13 asks for no leftovers. A choice in the wizard satisfies both, and Keep protects lists that weren't exported. `postuninst` also runs during an upgrade, so it deletes only when `SYNOPKG_PKG_STATUS` is `UNINSTALL` and `wizard_delete_data` is `true`. It refuses any folder whose path doesn't end in `vtunerradio/var`, and follows the folder if DSM made it a symlink. `postuninst` no longer reads `common`, so it can't fail if DSM has already removed other scripts.

### D3. start-stop-status follows the DSM 7 action list
DSM calls `prestart` and `prestop` because `precheckstartstop` defaults to yes. They return 0 because the package has nothing to check. Status 1 (dead with pid file) tells DSM the app crashed instead of reporting it as stopped.

### D4. Release links stay out of the repository
The project's web address contains an account name, and the repository holds no user names. The build adds the links only when `PROJECT_URL` is set, for example in a release build.

### D5. Try without the system group
The guide doesn't document `join-groupname` and asks packages to keep privileges low. It was added so that the admin CGI can run DSM's `authenticate.cgi`, but it was never confirmed that it fixed the sign-in. On 2026-11-06 the user chose to remove it in build 0007 and test on the NAS; if the admin UI then can't confirm the DSM login, it goes back in (build 0006 still has it) and the login trace shows what the CGI lacks.

### D6. Catalog version in its own header
The save check compared `If-Match` with the stored version. Seen on the NAS on 2026-11-06: saving failed with "changed in another window" with no other window open. nginx turns a strong ETag into a weak `W/"..."` one when it gzips a response, and the server then never matched. This cause is inferred, not traced on the NAS. Custom headers pass through unchanged, so the version travels in `X-Catalog-Version`; `If-Match` still works, weak or strong, for other clients. A save without a version is an explicit overwrite, which the UI only sends after the user confirms it on a real conflict.

## Risks / Trade-offs

- If DSM does remove `var` on uninstall, Keep doesn't keep anything and the wizard text is wrong. Task 3.3 checks this on the NAS before a release.
- The `singleselect` wizard type and its `true`/`false` keys follow the classic wizard format; the guide's 7.2.2 page documents only the newer custom render format. Task 3.2 checks it on the NAS.
