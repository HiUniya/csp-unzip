# CSP 双层解压工具

适用于 Windows 10 / 11。下载 Windows ZIP 并解压，将其中的 `请打开以解压压缩包.exe` 和 `csp-unzip.cfg` 与外层压缩包放在同一目录。EXE 内含便携 7-Zip，运行时离线解压。[下载 Windows 发行版](https://github.com/HiUniya/csp-unzip/releases/latest)

## 使用

1. 用记事本打开 `csp-unzip.cfg`，将 `outer_archive` 和 `inner_archive` 改为实际包名，以 UTF-8（无 BOM）保存。文件名中有空格时也直接填写。
2. 双击 `请打开以解压压缩包.exe`，先输入外层密码，外层解开后再输入内层密码。密码会显示在窗口中，按回车提交。密码错误时，按提示重输当前层密码即可。重试内层时会直接使用已经解出的内层包。输入中文标点（如 `，`、`！`）时，程序会提示切换英文输入法。
3. 成功后，在 EXE 同级的 `down` 加时间戳目录中查看解压文件，在 EXE 旁的 `problem.extern` 中查看本次题名清单。例如，`down20261124_143052` 表示 2026 年 11 月 24 日 14 时 30 分 52 秒的结果。同一秒再次解压时，目录名会依次加上 `_2`、`_3`。

```ini
outer_archive=试题包.7z
inner_archive=测试数据.zip
```

内层包名取自外层解压结果。只填文件名时会递归查找，若多个目录含有同名包，可填写包内相对路径，如 `复赛/测试数据.zip`。

按 `Ctrl+C` 可退出密码重试。如果确认密码无误却一直收到重输提示，请检查压缩包是否完整。7-Zip 对部分加密数据损坏也会给出相同的密码提示。

每次成功解压都会保留一个独立的结果目录，并用本次识别出的全部题名重写 EXE 旁的 `problem.extern`。解压或扫描失败时保留上次成功的清单。旧版 cfg 可以直接使用，其中的 `output_dir` 会被忽略。

## 题名规则

程序递归扫描 `.in` 和 `.ans`，忽略扩展名大小写。从 `.in` 文件名提取英文题名并去掉数字，按题名忽略大小写去重、按字母排序。题名内的下划线和连字符可以保留。同目录同名的 `.ans` 计入配对统计，缺少答案也可以生成题名。如果文件名只有序号，窗口会列出该文件。全部文件都无法识别时会报告原因。

例如，下面的输入文件会生成两行清单。

```text
测试数据/A/sum01.in
测试数据/A/sum_02.in
测试数据/B/short_path1.in
```

```text
short_path
sum
```

清单使用 UTF-8 无 BOM 和 Windows 换行。

## 从源码构建

在 Windows PowerShell 中，从源码目录执行以下命令。Go 1.23 或更新版本即可，第三方解压组件已包含在源码目录中。

```powershell
$env:GOOS = 'windows'
$env:GOARCH = '386'
$env:CGO_ENABLED = '0'
go build -trimpath -buildvcs=false -ldflags="-s -w" -o dist/请打开以解压压缩包.exe .
```

发行目录只需 EXE 和同目录的 cfg。程序与内置 7-Zip 均使用 x86 构建，兼容 32 位 Windows 10 和常见的 x64 Windows 10 / 11。运行 `.\请打开以解压压缩包.exe --license` 可查看内置 7-Zip 的版权、许可及对应源码地址。[组件来源及校验值](internal/sevenzip/README.md)
