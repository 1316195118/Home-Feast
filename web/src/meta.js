const DEFAULT_META = {
  title: '家里吃点啥｜家庭点菜与用餐协作',
  description: '一家人一起挑菜、点菜、分工和看记录的私有家庭点菜系统，数据保存在自己的 NAS 或本机。',
}

export const PAGE_META = {
  today: {
    title: '家里吃点啥｜今天吃什么，全家一起定',
    description: '从 237 道家常菜里一起挑今天吃什么，实时看到谁点了什么，顺手安排掌勺、洗碗和购物清单。',
  },
  menu: {
    title: '菜谱｜237 道家常菜按食材和口味挑',
    description: '按菜系、冰箱食材、辣度和耗时找菜，查看精确配料与做法，一键加入全家共享的今日菜单。',
  },
  family: {
    title: '家人｜分工排班与口味偏好一目了然',
    description: '管理家庭成员、忌口和每周排班，查看谁常掌勺、谁常洗碗，让家庭用餐分工更公平清楚。',
  },
  history: {
    title: '用餐记录｜回看每天点了什么、谁掌勺',
    description: '按日期回看家庭点菜与分工记录，把喜欢的一桌菜一键加回今天，也能快速找到常吃的家常菜。',
  },
  settings: {
    title: '设置｜隐私、数据和家庭偏好',
    description: '管理家庭名称、餐次、配图清晰度、备份恢复、访问口令和在线更新，数据始终留在自己的设备。',
  },
  privacy: {
    title: '隐私政策｜家里吃点啥',
    description: '了解家里吃点啥会保存哪些家庭数据、数据存放在哪里、如何使用、如何导出删除，以及当前的第三方服务情况。',
  },
  terms: {
    title: '服务条款｜家里吃点啥',
    description: '查看家里吃点啥的使用规则、家庭内容责任、知识产权、免责声明、服务终止和争议处理方式。',
  },
  notFound: {
    title: '页面未找到｜家里吃点啥',
    description: '这个页面可能被撤掉或地址输错了。回到今天、去菜谱挑菜，或查看家人与用餐记录。',
  },
}

function ensureMeta(attribute, key) {
  let element = document.head.querySelector(`meta[${attribute}="${key}"]`)
  if (!element) {
    element = document.createElement('meta')
    element.setAttribute(attribute, key)
    document.head.appendChild(element)
  }
  return element
}

function setMeta(attribute, key, content) {
  ensureMeta(attribute, key).setAttribute('content', content)
}

function setCanonical(url) {
  let link = document.head.querySelector('link[rel="canonical"]')
  if (!link) {
    link = document.createElement('link')
    link.setAttribute('rel', 'canonical')
    document.head.appendChild(link)
  }
  link.setAttribute('href', url)
}

export function applyPageMeta(route, tab = 'today') {
  const key = route === 'privacy' || route === 'terms'
    ? route
    : route === '404'
      ? 'notFound'
      : (PAGE_META[tab] ? tab : 'today')
  const meta = PAGE_META[key] || DEFAULT_META
  document.title = meta.title
  setMeta('name', 'description', meta.description)
  setMeta('property', 'og:title', meta.title)
  setMeta('property', 'og:description', meta.description)
  setMeta('property', 'og:url', window.location.href)
  setMeta('property', 'og:site_name', '家里吃点啥')
  setMeta('property', 'og:locale', 'zh_CN')
  setMeta('property', 'og:type', route === 'privacy' || route === 'terms' ? 'article' : 'website')
  setMeta('name', 'twitter:title', meta.title)
  setMeta('name', 'twitter:description', meta.description)
  setMeta('name', 'twitter:card', 'summary_large_image')
  setMeta('property', 'og:image', new URL('/og-cover.png', window.location.origin).href)
  setMeta('name', 'twitter:image', new URL('/og-cover.png', window.location.origin).href)
  setMeta('name', 'twitter:image:alt', '家里吃点啥：今天吃什么，全家一起定')
  setCanonical(window.location.href.split('#')[0])
}
