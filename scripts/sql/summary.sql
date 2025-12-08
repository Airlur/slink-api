-- 按日统计表 (stats_daily)
-- 记录每个短链接每天的总点击次数
CREATE TABLE IF NOT EXISTS `stats_daily` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `short_code` VARCHAR(20) NOT NULL COMMENT '关联短码',
  `date` DATE NOT NULL COMMENT '统计日期',
  `clicks` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日点击量',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code_date` (`short_code`, `date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='按日点击量统计表';

-- 按地域每日统计表 (stats_region_daily)
CREATE TABLE IF NOT EXISTS `stats_region_daily` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `short_code` VARCHAR(20) NOT NULL COMMENT '关联短码',
  `date` DATE NOT NULL COMMENT '统计日期',
  `province` VARCHAR(50) NOT NULL COMMENT '省份',
  `city` VARCHAR(50) NOT NULL COMMENT '城市',
  `clicks` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日该地区点击量',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code_date_province_city` (`short_code`, `date`, `province`, `city`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='按地域每日点击量统计表';

-- 按设备每日统计表 (stats_device_daily)
CREATE TABLE IF NOT EXISTS `stats_device_daily` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `short_code` VARCHAR(20) NOT NULL COMMENT '关联短码',
  `date` DATE NOT NULL COMMENT '统计日期',
  `device_type` VARCHAR(20) NOT NULL COMMENT '设备类型',
  `os_version` VARCHAR(50) NOT NULL COMMENT '操作系统',
  `browser` VARCHAR(50) NOT NULL COMMENT '浏览器',
  `clicks` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日该设备组合点击量',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code_date_device_os_browser` (`short_code`, `date`, `device_type`, `os_version`, `browser`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='按设备每日点击量统计表';