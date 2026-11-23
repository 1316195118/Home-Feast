<script setup>
defineEmits(['home', 'terms'])

const updatedAt = '2026年11月23日'
</script>

<template>
  <article class="legal-page">
    <header class="legal-hero">
      <span class="legal-eyebrow">PRIVACY · 隐私政策</span>
      <h1>你的家庭数据，默认只留在你家</h1>
      <p>
        家里吃点啥是一个自部署的家庭点菜工具。它没有广告、用户画像或第三方统计 SDK；
        菜谱、成员、点餐记录和图片保存在部署者自己的数据目录中。
      </p>
      <div class="legal-meta">
        <span>生效日期：{{ updatedAt }}</span>
        <span>适用于当前版本的自部署站点</span>
      </div>
    </header>

    <div class="legal-summary">
      <div><b>保存在服务器</b><span>成员、菜单、记录、冰箱食材和上传图片</span></div>
      <div><b>保存在本机</b><span>当前身份、主题、字号、声音和清单勾选状态</span></div>
      <div><b>不用于营销</b><span>无广告、无用户画像、无第三方行为追踪</span></div>
    </div>

    <nav class="legal-toc" aria-label="隐私政策目录">
      <a href="#privacy-scope">适用范围</a>
      <a href="#privacy-data">保存内容</a>
      <a href="#privacy-purpose">使用目的</a>
      <a href="#privacy-sharing">共享与第三方</a>
      <a href="#privacy-security">安全与保留</a>
      <a href="#privacy-rights">你的选择</a>
      <a href="#privacy-contact">联系与变更</a>
    </nav>

    <section id="privacy-scope" class="legal-section">
      <h2>1. 谁负责这些数据</h2>
      <p>
        本项目的服务端和数据库由站点部署者管理。通常部署者就是家庭设备的所有者；如果是单位或他人代为部署，
        该部署者负责决定数据如何保存、备份和删除。本站没有集中运营的云端账户系统。
      </p>
    </section>

    <section id="privacy-data" class="legal-section">
      <h2>2. 我们会接触哪些数据</h2>
      <div class="legal-grid">
        <div class="legal-card">
          <h3>家庭成员与偏好</h3>
          <p>成员名称、头像、颜色、忌口、餐次设置、家庭名称、冰箱食材和加入日期。</p>
        </div>
        <div class="legal-card">
          <h3>点餐与协作记录</h3>
          <p>日期、菜品、点菜人、餐次、留言、评分、掌勺与洗碗安排、骰子分工记录。</p>
        </div>
        <div class="legal-card">
          <h3>自定义菜谱与图片</h3>
          <p>菜名、菜系、配料用量、步骤、描述、标签，以及主动上传的菜品照片。</p>
        </div>
        <div class="legal-card">
          <h3>安全信息</h3>
          <p>可选访问口令、成员 PIN 的随机盐与哈希、口令失败次数和来源 IP；来源 IP 只在内存中用于短时防爆破。</p>
        </div>
      </div>
      <p>
        浏览器本机还会保存当前成员、访问口令、主题、字号、声音、购物清单勾选和 PWA 离线缓存。
        这些偏好不会进入家庭业务数据接口。
      </p>
    </section>

    <section id="privacy-purpose" class="legal-section">
      <h2>3. 为什么处理这些数据</h2>
      <ul>
        <li>展示菜谱、推荐今日菜品、合并购物清单并记录家庭选择。</li>
        <li>同步多人点菜和分工，避免重复操作和冲突。</li>
        <li>保存访问口令、成员 PIN、自动备份和在线更新状态。</li>
        <li>在设备本机记住显示和操作偏好，改善下一次访问体验。</li>
      </ul>
      <p>这些数据不用于广告投放、出售、信用评估或与家庭用餐无关的用户画像。</p>
    </section>

    <section id="privacy-sharing" class="legal-section">
      <h2>4. 什么时候会共享数据</h2>
      <p>
        默认不会向任何第三方提供家庭数据。只有在你主动操作时，数据才会经由浏览器或服务器外发：
      </p>
      <ul>
        <li>保存、分享或复制菜单图、购物清单图和 Excel 时，由你选择的系统分享目标接收。</li>
        <li>在线更新会由服务器访问 GitHub 获取版本文件，不上传成员、菜单或图片；若配置代理，代理运营方可能看到网络元数据。</li>
        <li>部署者另行接入反向代理、监控或分析工具时，其收集范围由该部署配置决定。</li>
      </ul>
    </section>

    <section id="privacy-security" class="legal-section">
      <h2>5. 安全、保留与删除</h2>
      <p>
        服务端使用 JSON 文件保存业务数据，访问口令默认不随业务快照或备份导出；成员 PIN 不保存明文。
        建议在家庭网络内使用，或启用可信 HTTPS 后再暴露到公网，并定期导出备份。
      </p>
      <p>
        数据会一直保存在部署目录，直到家庭成员删除内容、恢复备份或部署者删除对应卷。
        自动备份默认保留最近若干份，可在设置中调整；浏览器本地数据可通过清除站点数据删除。
        删除服务器数据卷属于不可恢复操作，请先导出备份。
      </p>
    </section>

    <section id="privacy-rights" class="legal-section">
      <h2>6. 你可以做什么</h2>
      <ul>
        <li>在家人页修改或删除成员及忌口，在记录页撤掉点菜记录。</li>
        <li>在菜谱页修改或删除自定义菜品和照片，在设置页清空冰箱食材。</li>
        <li>导出备份、关闭自动备份，或清除浏览器本地存储与离线缓存。</li>
        <li>联系部署者删除整个实例的数据目录；此操作前应先确认备份。</li>
      </ul>
    </section>

    <section class="legal-section">
      <h2>7. Cookie 与儿童使用</h2>
      <p>
        当前版本不使用广告或行为追踪 Cookie，也没有统计脚本。家庭可以为未成年成员建立身份，
        相关数据应由监护人录入和管理；服务不面向儿童投放广告。
      </p>
    </section>

    <section id="privacy-contact" class="legal-section">
      <h2>8. 联系与政策更新</h2>
      <p>
        隐私相关请求请联系本站部署者或家庭管理员。本项目没有统一的云端客服联系方式，
        因此页面上不虚构邮箱或公司地址。功能或数据范围发生实质变化时，会更新本页生效日期。
      </p>
    </section>

    <footer class="legal-actions">
      <button class="btn btn-primary" type="button" @click="$emit('home')">回到今天</button>
      <button class="btn btn-soft" type="button" @click="$emit('terms')">查看服务条款</button>
    </footer>
  </article>
</template>
