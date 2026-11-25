project(
  'dissent',
  version: '0.0.50',
  meson_version: '>= 0.59.0',
)

base_id = 'so.libdb.dissent'
is_devel = get_option('profile') == 'development'
app_id = is_devel ? base_id + '.Devel' : base_id

prefix = get_option('prefix')
bindir = prefix / get_option('bindir')
datadir = prefix / get_option('datadir')

go = find_program('go', '/usr/lib/sdk/golang/bin/go')
sed = find_program('sed')

dependency('gtk4')
dependency('libadwaita-1')
dependency('gtksourceview-5')
dependency('webkitgtk-6.0')
dependency('libspelling-1')

go_env = environment()
go_env.set('GOPATH', meson.current_build_dir() / '.gopath')
go_env.set('GOCACHE', meson.current_build_dir() / '.gocache')
go_env.append('GOFLAGS', '-modcacherw', separator: ' ')

dissent_bin = custom_target(
  'dissent',
  output: 'dissent',
  build_by_default: true,
  build_always_stale: true,
  console: true,
  env: go_env,
  install: true,
  install_dir: bindir,
  command: [
    go,
    'build',
    '-C', meson.current_source_dir(),
    '-v',
    '-ldflags', '-X main.AppID=' + app_id,
    '-o', meson.current_build_dir() / '@OUTPUT@',
    '.',
  ],
)

custom_target(
  app_id + '.service',
  input: 'nix' / (base_id + '.service'),
  output: app_id + '.service',
  build_by_default: true,
  capture: true,
  install: true,
  install_dir: datadir / 'dbus-1' / 'services',
  command: [
    sed,
    '-e', 's|^Name=' + base_id + '$|Name=' + app_id + '|',
    '-e', 's|/usr/bin/dissent|' + bindir + '/dissent|',
    '@INPUT@',
  ],
)

desktop_sed = [
  '-e', 's|^Icon=' + base_id + '$|Icon=' + app_id + '|',
]
if is_devel
  desktop_sed += ['-e', 's|^Name=Dissent$|Name=Dissent (Devel)|']
endif

custom_target(
  app_id + '.desktop',
  input: 'nix' / (base_id + '.desktop'),
  output: app_id + '.desktop',
  build_by_default: true,
  capture: true,
  install: true,
  install_dir: datadir / 'applications',
  command: [sed] + desktop_sed + ['@INPUT@'],
)

install_data(
  'internal/icons/hicolor/scalable/apps' / (app_id + '.svg'),
  install_dir: datadir / 'icons' / 'hicolor' / 'scalable' / 'apps',
)

install_data(
  'internal/icons/symbolic/apps' / (base_id + '-symbolic.svg'),
  rename: app_id + '-symbolic.svg',
  install_dir: datadir / 'icons' / 'hicolor' / 'symbolic' / 'apps',
)

custom_target(
  app_id + '.metainfo.xml',
  input: base_id + '.metainfo.xml',
  output: app_id + '.metainfo.xml',
  build_by_default: true,
  capture: true,
  install: true,
  install_dir: datadir / 'metainfo',
  command: [
    sed,
    '-e', 's|<id>' + base_id + '</id>|<id>' + app_id + '</id>|',
    '-e', 's|<launchable type="desktop-id">' + base_id + '.desktop</launchable>|<launchable type="desktop-id">' + app_id + '.desktop</launchable>|',
    '@INPUT@',
  ],
)

test(
  'go-test',
  go,
  env: go_env,
  args: ['test', '-C', meson.current_source_dir(), './...'],
)
