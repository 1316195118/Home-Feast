<script setup>
defineEmits(['home', 'privacy'])

const updatedAt = '2026年11月23日'
</script>

<template>
  <article class="legal-page">
    <header class="legal-hero">
      <span class="legal-eyebrow">TERMS · 服务条款</span>
      <h1>先把规则说清楚，再一起好好吃饭</h1>
      <p>
        家里吃点啥提供家庭菜谱浏览、点菜、分工、记录和备份功能。
        访问或使用本站，即表示你理解并同意以下规则。
      </p>
      <div class="legal-meta">
        <span>生效日期：{{ updatedAt }}</span>
        <span>适用于当前自部署实例</span>
      </div>
    </header>

    <nav class="legal-toc" aria-label="服务条款目录">
      <a href="#terms-service">服务范围</a>
      <a href="#terms-use">使用规范</a>
      <a href="#terms-content">家庭内容</a>
      <a href="#terms-ip">知识产权</a>
      <a href="#terms-disclaimer">免责声明</a>
      <a href="#terms-end">终止与争议</a>
    </nav>

    <section id="terms-service" class="legal-section">
      <h2>1. 服务范围</h2>
      <p>
        本站是自部署软件，不提供集中云端账户，也不承诺永久在线。部署者负责服务器、域名、网络、
        证书、备份和访问权限。具体功能和可用性以当前实例为准。
      </p>
    </section>

    <section id="terms-use" class="legal-section">
      <h2>2. 账户与使用规范</h2>
      <ul>
        <li>家庭成员身份由部署者或家庭管理员创建，不应冒用他人身份或共享敏感 PIN。</li>
        <li>不得利用本站攻击服务、绕过访问控制、批量探测口令、上传恶意文件或干扰其他家庭设备。</li>
        <li>不得未经授权将公开暴露的实例用于违法、侵权、骚扰或危害食品安全的行为。</li>
        <li>发现安全漏洞时，应私下通知部署者，不公开利用或破坏数据。</li>
      </ul>
    </section>

    <section id="terms-content" class="legal-section">
      <h2>3. 家庭内容与责任</h2>
      <p>
        成员名称、留言、自定义菜谱、图片和备份由使用者录入。录入者应确保拥有必要权利，
        并对内容的准确性、合法性和隐私负责。部署者可以在必要时删除违法、侵权或影响服务安全的内容。
      </p>
    </section>

    <section id="terms-ip" class="legal-section">
      <h2>4. 知识产权</h2>
      <p>
        项目源码按仓库中的 MIT 许可证授权；品牌图标、界面、项目文案和自生成图片的授权边界以
        <code>LICENSE</code> 与 <code>NOTICE.md</code> 为准。第三方依赖和公共领域用量数据保留各自声明。
        使用者自行上传的家庭照片和自写菜谱仍归原权利人所有。
      </p>
    </section>

    <section id="terms-disclaimer" class="legal-section">
      <h2>5. 饮食与数据免责声明</h2>
      <p>
        菜谱、用量和建议仅作家庭做法参考，未经专业营养、医疗或食品安全审核。
        过敏、忌口、生食、火候和食材新鲜度请自行判断。软件按“现状”提供，
        不保证数据永不丢失、服务永不中断或适合特定目的。
      </p>
      <p>
        请自行启用备份并保护访问口令。因硬盘损坏、误删、错误恢复、网络暴露、
        未授权访问或照做菜谱造成的损失，由使用者或部署者按其责任范围承担。
      </p>
    </section>

    <section id="terms-end" class="legal-section">
      <h2>6. 服务终止与争议处理</h2>
      <p>
        部署者可随时停止实例、删除数据或限制访问；使用者应提前导出需要保留的备份。
        发生争议时，各方应先保存日志和备份并通过友好协商解决；协商不成，按部署地适用法律和有管辖权的机构处理。
        本条款不排除消费者依法享有的不可放弃权利。
      </p>
    </section>

    <section class="legal-section">
      <h2>7. 条款更新</h2>
      <p>功能、法律要求或部署方式发生变化时，本页会更新。继续使用更新后的服务，表示接受修订后的条款。</p>
    </section>

    <footer class="legal-actions">
      <button class="btn btn-primary" type="button" @click="$emit('home')">回到今天</button>
      <button class="btn btn-soft" type="button" @click="$emit('privacy')">查看隐私政策</button>
    </footer>
  </article>
</template>
