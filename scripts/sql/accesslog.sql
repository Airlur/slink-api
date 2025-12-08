-- 访问日志模板表（access_logs_template）
CREATE TABLE IF NOT EXISTS `access_logs_template` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '日志唯一标识',
  `short_code` VARCHAR(20) NOT NULL COMMENT '关联短码',
  `user_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '访问者用户ID（非登录用户为0）',
  `ip` VARCHAR(45) NOT NULL COMMENT '访问IP地址',
  `user_agent` VARCHAR(512) NULL DEFAULT NULL COMMENT '设备User-Agent',
  `device_type` VARCHAR(20) NULL DEFAULT NULL COMMENT '设备类型（PC/Mobile/Tablet/Other）',
  `os_version` VARCHAR(50) NULL DEFAULT NULL COMMENT '操作系统版本',
  `browser` VARCHAR(50) NULL DEFAULT NULL COMMENT '浏览器类型',
  `province` VARCHAR(50) NULL DEFAULT NULL COMMENT '访问地域（省份）',
  `city` VARCHAR(50) NULL DEFAULT NULL COMMENT '访问地域（城市）',
  `channel` VARCHAR(50) NULL DEFAULT NULL COMMENT '来源渠道（直接访问/微信/微博等）',
  `accessed_at` DATETIME NOT NULL COMMENT '访问时间',
  PRIMARY KEY (`id`),
  INDEX `idx_short_code_accessed_at` (`short_code`, `accessed_at`),
  INDEX `idx_accessed_at` (`accessed_at`),
  INDEX `idx_province_city` (`province`, `city`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='访问日志模板表';
