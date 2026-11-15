%global goipath github.com/ublue-os/%{name}
%global debug_package %{nil}

Name:           uupd
Version:        1.5.7
Release:        1%{?dist}
Summary:        Centralized update service/checker made for Universal Blue
Vendor:         ublue-os
URL:            https://github.com/%{vendor}/%{name}
# Detailed information about the source Git repository and the source commit
# for the created rpm package
VCS:            {{{ git_dir_vcs }}}
# git_dir_pack macro places the repository content (the source files) into a tarball
# and returns its filename. The tarball will be used to build the rpm.
Source:         {{{ git_dir_pack }}}
License:        Apache-2.0

BuildRequires:  golang
BuildRequires:  systemd-rpm-macros
Recommends:     bootc
Recommends:     distrobox
Recommends:     flatpak
Requires:       libnotify
Requires:       systemd
Provides:       %{name} = %{version}

%description
%{summary}.

%gopkg
%gometa -f

%prep
{{{ git_dir_setup_macro }}}

%build
%define gomodulesmode GO111MODULE=on
%define currentgoldflags -X %{goipath}/cmd.version=%{version}
%gobuild -o output/%{name}

%install
%make_install

%check
%make_build test

%post
%systemd_post %{name}.timer

%preun
%systemd_preun %{name}.timer

%files
%license LICENSE
%doc README.md
%{_bindir}/%{name}
%{_unitdir}/%{name}.service
%{_unitdir}/%{name}.timer
%{_unitdir}/%{name}-manual.service
%{_datadir}/polkit-1/rules.d/%{name}.rules
%ghost %config(noreplace) %{_sysconfdir}/%{name}/config.json
%config(noreplace) %{_prefix}/lib/%{name}/config.json
%changelog
%autochangelog
