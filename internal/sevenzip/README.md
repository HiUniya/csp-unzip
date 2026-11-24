# 捆绑的 7-Zip

程序内嵌 Igor Pavlov 发布的 Windows x86 版 7-Zip 26.06。运行时解出 `7z.exe` 与同目录的 `7z.dll`。两者均取自 2026-11-06 的官方发行包，保留原始字节。[官方发行版](https://github.com/ip7z/7zip/releases/tag/26.06) · [对应版本源码](https://github.com/ip7z/7zip/releases/download/26.06/7z2606-src.tar.xz)

| 文件 | SHA-256 |
| --- | --- |
| 官方安装包 `7z2606.exe` | `5e38f80c3752de43887e7a99ad10668c14bf64dcc929a636510b54616ea7dec7` |
| `windows-386/7z.exe` | `decacebbb5855e69f70874836c69c90a4f589ea7b13772b90c90bd3c61da0edf` |
| `windows-386/7z.dll` | `af55cda37bb7c02b9dbc91786e33d2a86095ce83878fe79a820ea9e22ab1e5b1` |

安装包摘要已与官方 GitHub Release 资源元数据核对。使用 7-Zip 打开安装包即可提取并复核这两个文件。[安装包](https://github.com/ip7z/7zip/releases/download/26.06/7z2606.exe) · [发布元数据](https://api.github.com/repos/ip7z/7zip/releases/tags/26.06)

二进制的许可声明随程序内嵌。[7-Zip 原始声明](windows-386/License.txt) · [GNU LGPL 2.1](LGPL-2.1.txt)
