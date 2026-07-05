ssh lleng@192.168.1.55 /code/hubterm_project 

把现在的代码 整理提交。先review一下。

sudo systemctl start hubterm-center hubterm-agent && sleep 3 && sudo systemctl status hubterm-center hubterm-agent --no-pager -l 这个openclaw 部署的命令。你可以用 restart 来部署。 现在的改动没有其作用。
上次你提交的 agent ssh table 没有出来。
还是没有出来， 你能debug一下吗？ 不要应该。
http://192.168.1.55:8097/nodes/2f47b31f-9285-4d60-8392-6d7c08a35584
你说行给出证据。你到那步了？
有看到了。 不过能像 ssh 终端那样可以 保存配置， 也可以从里面取拿。 这个应该是同源的。
ssh 连接可以用。 ssh 终端 不可以用，点击 agent ssh 后， 就不能输入。

index-DC5HVA5d.js:21 ElementPlusError: [el-radio] [API] label act as value is about to be deprecated in version 3.0.0, please use value instead.
For more detail, please visit: https://element-plus.org/en-US/component/radio.html

    at Kt (index-DC5HVA5d.js:21:29673)
    at Dr.he.immediate (index-DC5HVA5d.js:21:43126)
    at _c (index-DC5HVA5d.js:13:38)
    at er (index-DC5HVA5d.js:13:109)
    at a.call (index-DC5HVA5d.js:13:3476)
    at y (index-DC5HVA5d.js:9:16657)
    at qC (index-DC5HVA5d.js:9:16871)
    at gp (index-DC5HVA5d.js:13:3677)
    at he (index-DC5HVA5d.js:13:3183)
    at Dr (index-DC5HVA5d.js:21:43107)

content.js:1 Uncaught (in promise) The message port closed before a response was received.

我希望 ssh 终端 是一个 table 。像下面的 在线 会话一样。 还有可以支持 在一个新的 tab 打开的按钮。 
保存的配置， 可以基于这个创建table的一条，创建的都需要入库。下次也可以用。
这个页面需要显示配置名称。
插件版和agent 版要一样。 现在插件版 没有改好。